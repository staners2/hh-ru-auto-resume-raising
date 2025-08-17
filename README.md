# hh-ru-auto-resume-raising

### Описание
Высокопроизводительная программа для автоматического подъема резюме на [HeadHunter](https://hh.ru/) 
каждые 4 часа. Альтернатива платной услуге 
[Продвижение.LITE](https://hh.ru/applicant/services/payment?from=landing&package=lite) 
от HeadHunter.

**🚀 Переписано на Go для оптимальной производительности:**
- Потребление памяти: ~20MB (вместо ~100MB+ Python)
- Размер Docker образа: ~15MB (вместо ~500MB)
- Время запуска: мгновенно (вместо 5-10 секунд)

### Архитектура проекта

```
hh-ru-auto-resume-raising/
├── cmd/hh-bot/              # Точка входа приложения
├── internal/                # Внутренние модули
│   ├── bot/                 # Telegram бот
│   ├── hh/                  # HH.ru API клиент
│   ├── scheduler/           # Планировщик задач
│   └── storage/             # Файловое хранилище
├── pkg/config/              # Конфигурация
├── .helm/                   # Helm чарт для Kubernetes
└── Dockerfile               # Multi-stage build
```

### Переменные окружения

Создайте файл `.env` со следующими переменными:

```bash
# Telegram настройки
TELEGRAM_TOKEN=your_bot_token_here
ADMIN_TG=your_telegram_user_id

# HeadHunter учетные данные
HH_LOGIN=your_hh_login
HH_PASSWORD=your_hh_password

# Дополнительные настройки
TZ=Europe/Moscow
PROXY=None  # или URL прокси сервера
```

### Локальный запуск

#### С помощью Go
```bash
# Установка зависимостей
go mod download

# Запуск
go run cmd/hh-bot/main.go
```

#### Сборка бинарного файла
```bash
# Сборка
go build -o hh-bot cmd/hh-bot/main.go

# Запуск
./hh-bot
```
### Запуск в контейнере
```
# Установить docker и docker-compose
# https://docs.docker.com/engine/install/ubuntu/

# Запустить приложение
docker compose up -d
```

### Развертывание в Kubernetes через Helm

#### Предварительные требования
- Kubernetes кластер
- Helm 3.x установлен

#### Установка
```bash
# Клонировать репозиторий
git clone https://github.com/staners2/hh-ru-auto-resume-raising.git
cd hh-ru-auto-resume-raising

# Установить чарт с базовыми настройками
helm install hh-bot .helm/

# Установить с кастомными настройками
helm install hh-bot .helm/ \
  --set env.HH_LOGIN="your_login" \
  --set env.HH_PASSWORD="your_password" \
  --set persistence.size=500Mi \
  --set persistence.storageClass=fast-ssd

# Обновить установку
helm upgrade hh-bot .helm/

# Удалить установку
helm uninstall hh-bot
```

#### Конфигурация через values.yaml
```bash
# Скопировать и отредактировать values.yaml
cp .helm/values.yaml my-values.yaml
# Отредактировать my-values.yaml с вашими настройками

# Установить с кастомным values.yaml
helm install hh-bot .helm/ -f my-values.yaml
```

#### Основные параметры конфигурации

**Переменные окружения:**
- `env.TELEGRAM_TOKEN` - токен Telegram бота
- `env.ADMIN_TG` - ID администратора в Telegram
- `env.HH_LOGIN` - логин от HeadHunter
- `env.HH_PASSWORD` - пароль от HeadHunter
- `env.TZ` - часовой пояс (по умолчанию `Europe/Moscow`)
- `env.PROXY` - прокси сервер (по умолчанию `None`)

**Ресурсы и хранилище:**
- `persistence.enabled` - включить Persistent Volume для хранения расписаний
- `persistence.size` - размер хранилища (по умолчанию 100Mi)
- `persistence.storageClass` - класс хранилища
- `resources.requests` - запрашиваемые ресурсы (CPU: 10m, Memory: 20Mi)
- `resources.limits` - лимиты ресурсов (CPU: 50m, Memory: 64Mi)

**Docker образ:**
- `image.repository` - Docker образ (по умолчанию `ghcr.io/staners2/hh-ru-auto-resume-raising`)
- `image.tag` - тег образа (по умолчанию `latest`)

#### Пример установки с переменными
```bash
helm install hh-bot .helm/ \
  --set env.TELEGRAM_TOKEN="your_bot_token" \
  --set env.ADMIN_TG="123456789" \
  --set env.HH_LOGIN="your_login" \
  --set env.HH_PASSWORD="your_password" \
  --set persistence.size=200Mi
```

### Принцип работы
1) Выполнить пункты из инструкции
2) Активировать бота (если бот был активирован ввести команду /start)
3) Нажать кнопку "Авторизация" (подгрузятся токены и сохранятся в файле config/tokens.json)
4) Нажать кнопку "Обновить список резюме" (подгрузятся резюме, в ответном сообщении наименования при нажатии сохраняются в буфер обмена)
5) Нажать кнопку "Добавить/обновить" и заполнить необходиме данные (в случае если запись уже существует, то она перезапишется с новыми данными)
6) Готово!
### Дополнительно 
- При поднятии придет уведомление в виде: наименование резюме, ответ запроса, время
- Кнопка "Расписание" (выведется список с динамическим расписанием, меняется в случае поднятия резюме)
- Кнопка "Список резюме" (локальный список, появляется после выполнения 4 пункта Принципа работы)
- Кнопка "Удалить" (далее ввести наименование резюме, которое нужно удалить из расписания)
- Кнопка "Профиль" (выведется список информации из файла .env)
- Кнопка "Вкл/выкл уведомления" (меняет состояние уведомлений о поднятии резюме)
### Подробнее об авторизации
- При нажатии на кнопку "Авторизоваться" токены создаются либо при их наличии обновляются.
- Если запущено расписание, то токены автоматически пересоздаются в случае разрыва сессии.
