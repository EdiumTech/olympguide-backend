# Развёртывание Go API в Yandex Cloud

Конфигурация для cloud `b1g3p4bfub6bklcrg72h`, folder `b1gum9p3p8pqr9jincsb`, адрес API `https://api.olympguide.ru/api/v1`.

**Развёрнуто 08.09.2026** в `cloud-arseniytitarenko93`: VM `olympguide-prod` (`fv4ksv7sq7gtm464uo7q`, `158.160.150.215`, `ru-central1-d`), релиз `20260908T120354Z-1ff7b8ba`. Публичные `/healthz`, `/readyz`, `/api/v1/universities` и `/api/v1/olympiads` возвращают HTTPS 200 с проверкой сертификата. Все контейнеры запущены, API и зависимости здоровы. Проверены повторная миграция без изменений, отказ анонимному созданию программы (401), резервное копирование и восстановление дампа в отдельную временную БД. Тесты Go API/storage и валидация Terraform/Compose проходят.

Публичная зона `olympguide.ru` (`dns7i4eqnqkkshne8hct`) перенесена из folder `b1gf8cc216t9an5p72ts` облака `cloud-ilhalatnikov` в целевой folder. При переносе сохранены все пять записей. После проверки нового сервера только A-запись `api.olympguide.ru` переключена на `158.160.150.215`, TTL 300; теперь ею управляет Terraform. NS у регистратора менять не требуется. Apex и `www` сохраняют прежний адрес `158.160.104.126`. Старая VM в другом облаке остановлена и не изменялась. Python viewer этим развёртыванием не публикуется.

Используется существующая default VPC `enpeqah6tdue9qeb6s28`: лимит числа сетей в облаке уже исчерпан. Для OlympGuide созданы отдельные подсеть `fl85oingi7ntgkjbk5u9` и security group `enp3pvd200e2u0rnq0u4`. Диск данных: `fv40dbamrj77atgjtanl`; статический IP: `fl85cb3d5j8lel2c8btg`; расписание снимков: `fd82peq5sah5hr7gpm70`. Другая VM в целевом folder не изменялась. Первоначальный адрес `84.201.168.22` заменён после подтверждённых тайм-аутов внешних SSH/HTTP-подключений; новый адрес проверен напрямую.

Для iOS используйте `BASE_URL = https://api.olympguide.ru/api/v1` (значение URL; в `.xcconfig` необходимо экранировать `//` по правилам этого формата). Текущая БД содержит 310 исходных олимпиад и 315 направлений из миграций, но **0 вузов и 0 льгот**. Набор 2026 года пока хранится отдельно в `olympguide-static`. SMTP не настроен: email worker выключен, письма регистрации не отправляются.

Итоговый полный `terraform plan` показывает `No changes`. Временная диагностическая VM, оба её диска и снимок удалены; временные правила firewall и доступ к serial console закрыты. Прямой SSH к новой VM проверен. Локальные state, параметры, ключ и секреты находятся в игнорируемых каталогах; они нужны для дальнейшего обслуживания.

## Что подготовлено

- Terraform: существующая или новая сеть, отдельные подсеть и security group, постоянный IPv4, Ubuntu 24.04 VM (2 vCPU, 4 GB RAM), загрузочный диск 25 GB, отдельный SSD-диск данных 40 GB, ежедневные снимки с хранением последних 7.
- Caddy выпускает и продлевает HTTPS-сертификат после переключения DNS. Из интернета доступны 80/443; SSH — только с заданных адресов. PostgreSQL, Redis, MinIO и gRPC не публикуют порты на хосте. `/metrics` и `/swagger` закрыты прокси.
- PostgreSQL 17, Redis 7.4, MinIO, Liquibase, API, storage и загрузчик дипломов. Email worker включается отдельно после настройки SMTP.
- Отдельный каталог данных `/srv/olympguide/data`, логические дампы БД в `/srv/olympguide/backups`, ежедневный запуск в 01:00 UTC перед снимком диска в 02:00 UTC.
- Случайные пароли и ключи подписи не попадают в Terraform, cloud-init или архив приложения. Они передаются по SSH в `/srv/olympguide/secrets/backend.env` с правами 0600.

Это стартовая конфигурация на одной VM: обновление контейнеров может дать короткий перерыв, отказ VM прерывает работу всех сервисов. VM, диски, постоянный IPv4 и снимки тарифицируются; фактическую стоимость нужно проверить для параметров итогового плана в облаке. Ежедневный снимок диска содержит также логический дамп; успешное восстановление следует проверить отдельно.

## 1. Авторизация и проверка существующих ресурсов

Команды ниже запускаются из корня `olympguide-backend`. Нужны Python 3.10+, OpenSSH, Terraform >= 1.12 и `yc`. В текущем workspace CLI установлены локально в `.tools`; для нового ПК используйте официальные инструкции установки.

```powershell
python deploy/deploy.py prepare
.\.tools\yc.exe --config .private/yc-config.yaml init
python deploy/deploy.py inventory
```

`prepare` создаёт `.private/backend.env` и выделенный SSH-ключ; существующие значения не перезаписывает. В `yc init` выберите указанные cloud/folder. OAuth-токен вводится только в локальном терминале, не в чат.

Вариант с авторизованным JSON-ключом сервисного аккаунта:

```powershell
.\.tools\yc.exe --config .private/yc-config.yaml config set service-account-key 'C:/absolute/path/authorized-key.json'
$env:YC_SERVICE_ACCOUNT_KEY_FILE = 'C:/absolute/path/authorized-key.json'
python deploy/deploy.py inventory
```

Сервисному аккаунту нужны права на создаваемые ресурсы Compute/VPC/DNS и чтение указанного каталога. Не храните ключ в отслеживаемых файлах. `inventory` сохраняет подробности только в `.private/inventory.json`: VM, адреса, диски, зоны и записи `olympguide.ru`.

Для текущей установки инвентаризация уже проведена: `158.160.104.126` принадлежит старой остановленной VM, а пользователь выбрал отдельную новую VM. При переносе другой существующей установки нужно сопоставить параметры с конфигурацией и импортировать ресурсы; импорт одной VM без её дисков, сети и IP недостаточен.

## 2. План инфраструктуры

В текущем workspace параметры уже находятся в игнорируемом `infra/yandex/terraform.tfvars.json`; не создавайте рядом дублирующий `.tfvars`. Для новой установки скопируйте `infra/yandex/terraform.tfvars.example` в `infra/yandex/terraform.tfvars`. Укажите реальный ID публичной DNS-зоны, содержимое `.private/deploy_ed25519.pub` и свой внешний IPv4 `/32` для SSH. Приватный ключ туда не вставлять. `existing_network_id` позволяет использовать сеть без создания ещё одной VPC.

```powershell
./deploy/terraform.ps1 init
./deploy/terraform.ps1 validate
./deploy/terraform.ps1 plan
```

Проверьте каждое создание/изменение/удаление в плане, особенно существующий IP, VM и диски. После проверки:

```powershell
./deploy/terraform.ps1 apply
./deploy/terraform.ps1 output
```

Обёртка получает краткоживущий IAM-токен через `yc`, передаёт в окружении только на время Terraform и применяет именно сохранённый `deploy.tfplan`. Токен в консоль не выводит.

На Linux/Mac используются те же `.tf` файлы:

```bash
export TF_CLI_CONFIG_FILE="$PWD/infra/yandex/terraform.rc"
export YC_TOKEN="$(yc iam create-token)"
terraform -chdir=infra/yandex init
terraform -chdir=infra/yandex plan -out=deploy.tfplan
terraform -chdir=infra/yandex apply deploy.tfplan
unset YC_TOKEN
```

Lock-файл содержит контрольные суммы провайдера для Windows x86_64, Linux x86_64 и Apple Silicon. Terraform state хранится локально и исключён из Git. Сохраните его и `.private` в защищённой резервной копии; без state повторный запуск с другого ПК попытается создать вторую инфраструктуру. Для команды следующий шаг — общий защищённый backend state с блокировкой.

## 3. Секреты и публикация приложения

При новой установке `.private/backend.env` уже содержит случайные значения. Для существующей БД сначала нужны её настройки и согласованная процедура переноса: смена `POSTGRES_PASSWORD` в env сама по себе пароль существующего PostgreSQL не меняет. Скрипт откажется заменять отличающийся env на сервере; ротация секретов выполняется отдельно.

Для писем добавьте SMTP-логин и пароль приложения, затем `COMPOSE_PROFILES=email`. Без этого регистрация и восстановление доступа через email не отправляют письма. Скрипт не посылает тестовых писем. OAuth-входы требуют отдельной проверки с iOS-клиентом.

Перед SSH подключением подтвердите ключ хоста через доверенный канал. `deploy/trust_host.py` читает ключ и его отпечаток из вывода serial console через авторизованный API Yandex Cloud и сохраняет `.private/known_hosts`; параметры команды доступны с `--help`. Скрипт использует `StrictHostKeyChecking=yes`. На Windows приватный ключ должен принадлежать текущему пользователю с закрытыми ACL; запуск SSH под другим системным пользователем может не иметь доступа к ключу.

```powershell
python deploy/deploy.py package
python deploy/deploy.py deploy --host 158.160.150.215
```

Загрузка проверяет завершение cloud-init и SHA256 архива, затем запускает отдельную systemd-службу `olympguide-deploy-<release>`. Её состояние сохраняется в `.private/deployment-job.json`; сборка продолжится при обрыве SSH. Служба собирает образы, делает дамп перед обновлением существующей установки, применяет Liquibase и ждёт готовности API. Архив собирается из явного списка папок приложения; `.git`, `.private`, `.env`, локальные инструменты и данные загрузчика не отправляются.

Состояние фонового запуска: `sudo systemctl status olympguide-deploy-<release>`; журнал: `sudo journalctl -u olympguide-deploy-<release> --no-pager`. При потере связи сначала проверьте существующий запуск, чтобы не начать второй параллельно. В Docker настроен официальный кеш Docker Hub `https://mirror.gcr.io`; пакеты Alpine в Dockerfile загружаются через `https://mirror.yandex.ru/mirrors/alpine`, поскольку исходные CDN были недоступны с VM. Buildx устанавливается cloud-init; Compose ограничивает параллелизм сборки.

Ошибки сборки и миграций останавливают запуск. Блокировка `flock` предотвращает два одновременных обновления. История Liquibase закреплена в `public`: создание схемы `olympguide`, совпадающей с именем пользователя PostgreSQL, иначе меняло неявную схему и приводило к повторному выполнению DDL. Автоматического отката схемы БД нет. При ошибке после миграции необходимо проверить её совместимость с предыдущим приложением перед откатом; `previous` — только ссылка на прошлую успешно опубликованную сборку. Не использовать `docker compose down -v` для обновлений.

## 4. DNS и HTTPS

После проверки приложения на VM включите `manage_dns_record = true`. Если A-запись уже существует, импортируйте её перед новым планом:

```powershell
.\.tools\terraform.exe -chdir=infra/yandex import 'yandex_dns_recordset.api[0]' 'ZONE_ID/api.olympguide.ru./A'
./deploy/terraform.ps1 plan
./deploy/terraform.ps1 apply
python deploy/deploy.py verify
```

Для прямого `terraform import` также нужен `YC_TOKEN` или `YC_SERVICE_ACCOUNT_KEY_FILE`. Не переносите NS, SOA, MX и другие записи зоны: Terraform управляет только A-записью API. При переключении со старой VM сохраните возможность возврата её адреса.

`verify` проверяет TLS с обычной валидацией сертификата, `/healthz`, `/readyz`, JSON университетов и олимпиад. `/readyz` возвращает 503 при недоступности PostgreSQL или Redis. Дополнительно проверьте авторизацию iOS, загрузку логотипов, почту, содержимое каталога и восстановление дампа.

## Данные 2026 года

Go API использует старую схему `olympguide` из `migrations/`. Миграции содержат справочники, но не новый набор вузов, факультетов и льгот.

Актуальный каталог и браузерный просмотр находятся в соседнем репозитории `olympguide-static`: `data_loader/admissions/snapshots/2026` и `web/`. Его PostgreSQL-экспорт создаёт отдельные таблицы `admission_*`; это не импорт в модели старого Go API. Данный deployment не переносит каталог 2026 автоматически и не публикует Python viewer. Для данных в Go API нужен отдельный адаптер с сохранением условий льгот и связей подразделений либо явная интеграция нового каталога. Нельзя выдавать агрегированные условия за точные льготы конкретной программы.

## Проверки и обслуживание

```powershell
go -C api test ./...
go -C storage_service test ./...
docker compose --env-file .private/backend.env -f deploy/compose.yaml config --quiet
terraform -chdir=infra/yandex fmt -check
terraform -chdir=infra/yandex validate
```

Контейнерный smoke test выполнен на новой БД. `deploy/verify.sh` проверяет API, запрет анонимной записи, повторную миграцию, создаёт дамп и восстанавливает его в отдельную временную БД; после сравнения числа сущностей удаляет только эту тестовую БД. Запускайте файл на VM как root. Не передавайте его напрямую в `bash -s`: Docker может прочитать оставшийся stdin скрипта. На VM состояние и логи смотрятся через `docker compose --env-file /srv/olympguide/secrets/backend.env -f /srv/olympguide/current/deploy/compose.yaml ps` / `logs --tail 100 api`. Не выводите `compose config` без `--quiet`: там будут секреты.

Дамп восстанавливается в отдельную PostgreSQL 17 БД через `pg_restore`, затем проверяется число сущностей и запросы API. Снимки обеспечивают восстановление диска; они не заменяют проверку логического восстановления. Данные диска и постоянный IP защищены `prevent_destroy` в Terraform, а диск не удаляется автоматически вместе с VM.

Документация: [Terraform в Yandex Cloud](https://yandex.cloud/ru/docs/tutorials/infrastructure-management/terraform-quickstart), [установка yc](https://yandex.cloud/ru/docs/cli/operations/install-cli), [снимки по расписанию](https://yandex.cloud/ru/docs/compute/operations/snapshot-control/create-schedule), [импорт DNS-записей](https://registry.terraform.io/providers/yandex-cloud/yandex/latest/docs/resources/dns_recordset).
