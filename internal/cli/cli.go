package cli

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"taskmgr/internal/config"
	"taskmgr/internal/storage"
	"taskmgr/internal/task"
)

type App struct {
	cfg   *config.Config
	store storage.Storage
	log   *slog.Logger
	out   io.Writer
}

func NewApp(cfg *config.Config, store storage.Storage, log *slog.Logger) *App {
	return &App{cfg: cfg, store: store, log: log, out: os.Stdout}
}

func (a *App) Run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		a.usage()
		return nil
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "add":
		return a.cmdAdd(ctx, rest)
	case "list":
		return a.cmdList(ctx, rest)
	case "done":
		return a.cmdDone(ctx, rest)
	case "delete":
		return a.cmdDelete(ctx, rest)
	case "stats":
		return a.cmdStats(ctx, rest)
	case "import":
		return a.cmdImport(ctx, rest)
	case "export":
		return a.cmdExport(ctx, rest)
	case "help", "-h", "--help":
		a.usage()
		return nil
	default:
		return fmt.Errorf("unknown command %q (see 'taskmgr help')", cmd)
	}
}

func (a *App) usage() {
	fmt.Fprintln(a.out, `taskmgr — console task manager

Usage:
  taskmgr [--config FILE] [--log-level LVL] <command> [args]

Commands:
  add <title> [--priority=low|medium|high] [--tags=a,b,c]
  list [--status=pending|done] [--sort=priority|created] [--tag=TAG]
  done <id>...
  delete <id>...
  stats
  import <file.json> [--workers=N]
  export <file> --format=json|csv
  help`)
}

func (a *App) cmdAdd(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	prio := fs.String("priority", "medium", "priority")
	tags := fs.String("tags", "", "comma separated tags")
	if err := fs.Parse(args); err != nil {
		return err
	}
	title := strings.Join(fs.Args(), " ")
	if title == "" {
		return errors.New("title is required")
	}
	p, err := task.ParsePriority(*prio)
	if err != nil {
		return err
	}
	tasks, err := a.store.Load(ctx)
	if err != nil {
		return err
	}
	t := &task.Task{
		ID:        tasks.NextID(),
		Title:     title,
		Priority:  p,
		Status:    task.Pending,
		Tags:      splitTags(*tags),
		CreatedAt: time.Now().UTC(),
	}
	tasks = append(tasks, t)
	if err := a.store.Save(ctx, tasks); err != nil {
		return err
	}
	a.log.Info("task added", "id", t.ID, "title", t.Title)
	fmt.Fprintf(a.out, "added #%d: %s\n", t.ID, t.Title)
	return nil
}

func (a *App) cmdList(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	status := fs.String("status", "", "pending|done")
	tag := fs.String("tag", "", "filter by tag")
	sortBy := fs.String("sort", "created", "priority|created")
	if err := fs.Parse(args); err != nil {
		return err
	}
	tasks, err := a.store.Load(ctx)
	if err != nil {
		return err
	}
	filtered := tasks.Filter(func(t *task.Task) bool {
		if *status != "" && string(t.Status) != *status {
			return false
		}
		if *tag != "" && !contains(t.Tags, *tag) {
			return false
		}
		return true
	})
	if *sortBy == "priority" {
		filtered.SortByPriority()
	}
	if len(filtered) == 0 {
		fmt.Fprintln(a.out, "no tasks")
		return nil
	}
	w := tabwriter.NewWriter(a.out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSTATUS\tPRIORITY\tTAGS\tTITLE")
	for _, t := range filtered {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n",
			t.ID, t.Status, t.Priority, strings.Join(t.Tags, ","), t.Title)
	}
	return w.Flush()
}

func (a *App) cmdDone(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: taskmgr done <id>...")
	}
	ids, err := parseIDs(args)
	if err != nil {
		return err
	}
	tasks, err := a.store.Load(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	var updated int
	for _, id := range ids {
		t := tasks.Find(id)
		if t == nil {
			a.log.Warn("task not found", "id", id)
			continue
		}
		if t.Status == task.Done {
			continue
		}
		t.Status = task.Done
		t.DoneAt = &now
		updated++
	}
	if updated == 0 {
		fmt.Fprintln(a.out, "nothing to update")
		return nil
	}
	if err := a.store.Save(ctx, tasks); err != nil {
		return err
	}
	fmt.Fprintf(a.out, "marked done: %d\n", updated)
	return nil
}

func (a *App) cmdDelete(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: taskmgr delete <id>...")
	}
	ids, err := parseIDs(args)
	if err != nil {
		return err
	}
	set := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	tasks, err := a.store.Load(ctx)
	if err != nil {
		return err
	}
	var kept task.List
	var removed int
	for _, t := range tasks {
		if _, ok := set[t.ID]; ok {
			removed++
			continue
		}
		kept = append(kept, t)
	}
	if removed == 0 {
		fmt.Fprintln(a.out, "nothing to delete")
		return nil
	}
	if err := a.store.Save(ctx, kept); err != nil {
		return err
	}
	fmt.Fprintf(a.out, "deleted: %d\n", removed)
	return nil
}

func (a *App) cmdStats(ctx context.Context, args []string) error {
	tasks, err := a.store.Load(ctx)
	if err != nil {
		return err
	}
	total := len(tasks)
	var done int
	byPrio := map[task.Priority]int{}
	for _, t := range tasks {
		if t.Status == task.Done {
			done++
		}
		byPrio[t.Priority]++
	}
	fmt.Fprintf(a.out, "total:    %d\n", total)
	fmt.Fprintf(a.out, "done:     %d\n", done)
	fmt.Fprintf(a.out, "pending:  %d\n", total-done)
	if total > 0 {
		fmt.Fprintf(a.out, "progress: %.1f%%\n", float64(done)/float64(total)*100)
	}
	fmt.Fprintln(a.out, "by priority:")
	for _, p := range []task.Priority{task.High, task.Medium, task.Low} {
		fmt.Fprintf(a.out, "  %-6s %d\n", p, byPrio[p])
	}
	return nil
}

func (a *App) cmdImport(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	workers := fs.Int("workers", 4, "number of worker goroutines")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: taskmgr import <file.json> [--workers=N]")
	}
	path := fs.Arg(0)
	if *workers < 1 {
		*workers = 1
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	var incoming []*task.Task
	if err := json.NewDecoder(f).Decode(&incoming); err != nil {
		return fmt.Errorf("decode input: %w", err)
	}

	tasks, err := a.store.Load(ctx)
	if err != nil {
		return err
	}
	next := tasks.NextID()

	type job struct {
		idx int
		in  *task.Task
	}
	jobs := make(chan job)
	out := make([]*task.Task, len(incoming))

	var wg sync.WaitGroup
	wg.Add(*workers)
	for i := 0; i < *workers; i++ {
		go func() {
			defer wg.Done()
			for j := range jobs {
				t := j.in
				t.ID = next + int64(j.idx)
				t.Title = strings.TrimSpace(t.Title)
				if t.Status == "" {
					t.Status = task.Pending
				}
				if t.CreatedAt.IsZero() {
					t.CreatedAt = time.Now().UTC()
				}
				out[j.idx] = t
			}
		}()
	}

	go func() {
		defer close(jobs)
		for i, t := range incoming {
			select {
			case <-ctx.Done():
				return
			case jobs <- job{idx: i, in: t}:
			}
		}
	}()
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return err
	}

	for _, t := range out {
		if t != nil {
			tasks = append(tasks, t)
		}
	}
	if err := a.store.Save(ctx, tasks); err != nil {
		return err
	}
	fmt.Fprintf(a.out, "imported: %d (workers=%d)\n", len(out), *workers)
	return nil
}

func (a *App) cmdExport(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	format := fs.String("format", "json", "json|csv")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: taskmgr export <file> --format=json|csv")
	}
	path := fs.Arg(0)

	tasks, err := a.store.Load(ctx)
	if err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	switch *format {
	case "json":
		enc := json.NewEncoder(f)
		enc.SetIndent("", "  ")
		if err := enc.Encode(tasks); err != nil {
			return err
		}
	case "csv":
		w := csv.NewWriter(f)
		_ = w.Write([]string{"id", "title", "priority", "status", "tags", "created_at"})
		for _, t := range tasks {
			_ = w.Write([]string{
				strconv.FormatInt(t.ID, 10),
				t.Title,
				t.Priority.String(),
				string(t.Status),
				strings.Join(t.Tags, "|"),
				t.CreatedAt.Format(time.RFC3339),
			})
		}
		w.Flush()
		if err := w.Error(); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported format %q", *format)
	}
	fmt.Fprintf(a.out, "exported %d tasks to %s\n", len(tasks), path)
	return nil
}

func parseIDs(args []string) ([]int64, error) {
	ids := make([]int64, 0, len(args))
	for _, s := range args {
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid id %q", s)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func splitTags(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
