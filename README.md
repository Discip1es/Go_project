# taskmgr

Консольный менеджер задач на Go. Один бинарник, никаких внешних зависимостей — только стандартная библиотека.

```
$ taskmgr add "Написать отчёт" --priority=high --tags=work,urgent
added #1: Написать отчёт

$ taskmgr list --sort=priority
ID  STATUS   PRIORITY  TAGS         TITLE
1   pending  high      work,urgent  Написать отчёт
2   pending  medium                 Купить хлеб
```

## Возможности

- **Подкоманды**: `add`, `list`, `done`, `delete`, `stats`, `import`, `export`
- **Приоритеты** (`low` / `medium` / `high`) и **теги**
- **Хранилище** — JSON-файл с атомарной записью (tmp + rename)
- **Конфиг** — JSON-файл в `~/.taskmgr/config.json` (переопределяется через `--config`)
- **Логи** — `log/slog` в stderr (text) или файл (JSON), уровень через `--log-level`
- **Импорт** — параллельная обработка через worker pool, отменяется по `Ctrl+C`
- **Экспорт** — в JSON или CSV
- **Graceful shutdown** по `SIGINT` / `SIGTERM` — незавершённые операции аккуратно отменяются через `context`

## Установка

Требуется Go 1.21+ (используется `log/slog` и `signal.NotifyContext`).

```bash
git clone <repo> taskmgr && cd taskmgr
go build -o taskmgr .
# опционально:
sudo install -m 0755 taskmgr /usr/local/bin/taskmgr
```

## Быстрый старт

```bash
taskmgr add "Купить хлеб"
taskmgr add "Написать отчёт" --priority=high --tags=work,urgent
taskmgr list
taskmgr done 1
taskmgr stats
```

## Использование

```
taskmgr [--config FILE] [--log-level LVL] <command> [args]
```

### Глобальные флаги

| Флаг | По умолчанию | Описание |
|---|---|---|
| `--config FILE` | `~/.taskmgr/config.json` | путь к конфигу |
| `--log-level LVL` | из конфига (`info`) | `debug` \| `info` \| `warn` \| `error` |

### Команды

#### `taskmgr add <title> [--priority=low|medium|high] [--tags=a,b,c]`

Создаёт задачу. Приоритет по умолчанию — `medium`.

```bash
taskmgr add "Сходить в зал" --priority=low --tags=health
taskmgr add Купить молоко --tags=home
```

#### `taskmgr list [--status=pending|done] [--tag=TAG] [--sort=created|priority]`

Выводит задачи таблицей. По умолчанию сортировка по времени создания.

```bash
taskmgr list
taskmgr list --status=pending --sort=priority
taskmgr list --tag=work
```

#### `taskmgr done <id>...`

Помечает одну или несколько задач выполненными. Идемпотентно.

```bash
taskmgr done 3
taskmgr done 1 2 5
```

#### `taskmgr delete <id>...`

Удаляет одну или несколько задач.

```bash
taskmgr delete 4
```

#### `taskmgr stats`

Сводка: всего / выполнено / в работе / прогресс / распределение по приоритетам.

```
total:    10
done:     4
pending:  6
progress: 40.0%
by priority:
  high   2
  medium 5
  low    3
```

#### `taskmgr import <file.json> [--workers=N]`

Импортирует массив задач из JSON. `id` во входном файле игнорируется — назначаются новые. Обработка параллельная (по умолчанию 4 воркера).

Формат входного файла — тот же, что у `export`:

```json
[
  {
    "title": "Задача из бэкапа",
    "priority": 2,
    "tags": ["imported"]
  },
  {
    "title": "Ещё одна",
    "priority": 0
  }
]
```

```bash
taskmgr import backup.json --workers=8
```

#### `taskmgr export <file> --format=json|csv`

Сохраняет все задачи в файл.

```bash
taskmgr export backup.json
taskmgr export report.csv --format=csv
```

#### `taskmgr help`

Краткая справка по всем командам.

## Конфигурация

Конфиг ищется в `~/.taskmgr/config.json` (или по `--config`). Отсутствие файла — не ошибка, применяются значения по умолчанию.

```json
{
  "data_file": "~/.taskmgr/tasks.json",
  "log_file":  "~/.taskmgr/taskmgr.log",
  "log_level": "info"
}
```

| Поле | Описание |
|---|---|
| `data_file` | путь к JSON-хранилищу задач |
| `log_file` | если задано — логи пишутся туда в формате JSON; иначе — текст в stderr |
| `log_level` | `debug` \| `info` \| `warn` \| `error` |

Флаг `--log-level` переопределяет значение из конфига.

## Формат данных

Задачи хранятся как JSON-массив:

```json
[
  {
    "id": 1,
    "title": "Написать отчёт",
    "priority": 2,
    "status": "pending",
    "tags": ["work", "urgent"],
    "created_at": "2025-01-15T10:30:00Z"
  },
  {
    "id": 2,
    "title": "Купить хлеб",
    "priority": 1,
    "status": "done",
    "created_at": "2025-01-15T10:31:00Z",
    "done_at": "2025-01-15T12:00:00Z"
  }
]
```

Кодировка приоритетов: `0` = low, `1` = medium, `2` = high. Поле `done_at` появляется только у выполненных задач.

Запись атомарна: сначала данные пишутся в `tasks.json.tmp`, затем переименовываются. Это защищает от порчи файла при аварийном завершении.

## Архитектура

```
taskmgr/
├── main.go                    # точка входа: флаги, логгер, context, signal
└── internal/
    ├── config/                # загрузка конфига
    ├── task/                  # домен: Task, List, Priority, Status
    ├── storage/               # интерфейс Storage + JSON-реализация
    └── cli/                   # парсинг подкоманд и рендеринг вывода
```

Зависимости направлены «внутрь»: `cli → storage → task`, `config` используется только в `main` и `cli`. Хранилище спрятано за интерфейсом `storage.Storage` — чтобы добавить, например, SQLite, достаточно реализовать `Load`/`Save` и передать новую реализацию в `cli.NewApp`.

Ключевые места, где используется `context.Context`:

- проброс из `main` во все команды — отмена по `Ctrl+C`;
- `import` — отменяемый worker pool;
- `storage.Load/Save` — ранний выход, если контекст уже отменён.

## Разработка

```bash
go build ./...       # сборка
go vet ./...         # статический анализ
go test ./...        # тесты (если будут)
```

Полезные точки расширения:

- **новое хранилище** — реализовать `storage.Storage` (например, SQLite через `modernc.org/sqlite`);
- **новая команда** — добавить case в `cli.App.Run` и метод `cmdXxx`;
- **новый формат экспорта** — ветка в `cli.App.cmdExport`;
- **тесты** — `task.List.Filter`, `SortByPriority` и `cli.cmdAdd` удобно покрывать, подсунув in-memory `Storage`.

## Лицензия

MIT — используйте как угодно.
