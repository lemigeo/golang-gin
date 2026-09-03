# golang-gin

Go + Gin REST API.

- **DB 접근**: sqlc — `.sql` 파일에서 타입 안전한 Go 코드를 생성한다. ORM은 쓰지 않는다.
- **마이그레이션**: golang-migrate
- **모듈 경로**: `golang-gin`


## 시작하기

```zsh
# 도구 설치 (최초 1회)
brew install go golangci-lint
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
go install -tags 'mysql' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# 의존성
go mod tidy

# 로컬 DB 기동 (127.0.0.1:3306 고정)
docker compose up -d --wait

# 마이그레이션 적용
source .env
migrate -path migrations -database "mysql://$DB_DSN" up

# 실행
go run ./cmd/api
```

`.env`가 없으면 `.env.example`을 복사해서 만든다. 테스트용은 `.env.test.example` → `.env.test`.

## 디렉터리 구조

```
├── cmd/
│   └── api/
│       └── main.go                  # 설정 로딩 + 서버 기동만
├── internal/
│   ├── config/                      # 환경변수, 설정 로딩
│   ├── handler/                     # Gin 핸들러 (HTTP layer)
│   │   ├── health_handler.go
│   │   ├── user_handler.go
│   │   └── router.go                # 라우팅 등록 + 엔진 조립
│   ├── service/                     # 비즈니스 로직 (usecase)
│   │   └── user_service.go
│   ├── repository/
│   │   └── mysql/                   # sqlc 생성 코드를 감싸는 얇은 레이어 (인터페이스 없음)
│   │       ├── sqlc/                # sqlc 생성 코드 — 손으로 수정 금지
│   │       │   ├── db.go
│   │       │   ├── models.go
│   │       │   └── user.sql.go
│   │       └── user_repository.go   # sqlc 호출 + domain 매핑
│   ├── domain/                      # 엔티티, DTO, 도메인 모델
│   │   └── user.go
│   ├── middleware/                  # 인증, 로깅, CORS, recover
│   │   └── auth.go
│   ├── pkg/                         # 내부 공용 유틸 (에러, 응답 포맷)
│   └── testutil/                    # 테스트 인프라 (컨테이너, 픽스처, API 클라이언트)
│       ├── db.go                    # 테스트 MySQL 기동 + 마이그레이션
│       ├── tx.go                    # 트랜잭션 격리 헬퍼
│       ├── app.go                   # 전체 앱 조립 (httptest)
│       └── fixture.go               # 테스트 데이터 생성
├── db/
│   └── query/                       # sqlc 입력 쿼리 (.sql)
│       └── user.sql
├── sqlc.yaml                        # sqlc 설정
├── pkg/                             # 외부 공개 가능한 공용 라이브러리
├── migrations/                      # DB 마이그레이션 = sqlc의 스키마 입력
├── test/
│   └── integration/                 # 엔드투엔드 API 테스트
│       ├── main_test.go
│       └── user_test.go
├── go.mod
└── go.sum
```

## 아키텍처

### 핵심 원칙: Repository 인터페이스를 두지 않는다

일반적인 클린 아키텍처와 달리, **손으로 쓴 repository 인터페이스 레이어를 만들지 않는다.**
Service는 구현체 구조체를 **직접** 참조한다.

```go
// internal/service/user_service.go
type UserService struct {
    userRepo *mysql.UserRepository // 인터페이스 아님. 구체 타입 직접 참조
}

func NewUserService(userRepo *mysql.UserRepository) *UserService {
    return &UserService{userRepo: userRepo}
}
```

금지 사항:

- `type UserRepository interface { ... }` 같은 repository 인터페이스 정의 금지
- 인터페이스 추상화를 위한 mock 생성기(mockery, gomock) 도입 금지
- "나중에 DB 교체를 대비해서" 같은 이유로 인터페이스 추가 제안 금지

예외: sqlc가 생성하는 `DBTX` 인터페이스는 생성 코드의 일부이므로 그대로 쓴다. 이건 손으로 만드는 추상화가 아니다.

인터페이스가 없으므로 테스트는 mock이 아니라 **실제 MySQL을 띄운 통합 테스트**로 검증한다. 아래 「테스트」 항목을 따른다.

### 레이어 규칙

의존 방향은 한 방향으로만 흐른다:

```
handler → service → repository/mysql → sqlc → DB
             ↘ domain ↙
```

| 레이어 | 하는 일 | 하면 안 되는 일 |
|---|---|---|
| `handler` | 요청 바인딩/검증, service 호출, 응답 변환 | SQL 작성, 비즈니스 규칙 판단 |
| `service` | 비즈니스 로직, 트랜잭션 경계 결정 | `*gin.Context` 참조, HTTP 상태코드 결정, sqlc 타입 직접 사용 |
| `repository/mysql` | sqlc 쿼리 호출, sqlc row → domain 매핑, DB 에러 → domain 에러 변환 | 비즈니스 규칙, 권한 판단 |
| `sqlc/` | 생성 코드 | 손으로 수정 |
| `domain` | 엔티티, DTO, 도메인 에러 | 다른 internal 패키지 import |

- `service`는 `*gin.Context`를 받지 않는다. 표준 `context.Context`만 받는다.
- **sqlc 생성 타입은 `repository/mysql` 바깥으로 새어 나가지 않는다.** service와 handler는 `domain` 타입만 본다.
- 역방향 import(예: `repository` → `service`) 금지.

### DI 조립

와이어링은 수동으로만 한다. wire/fx 같은 DI 프레임워크 도입 금지.

**조립은 `main.go`가 아니라 `handler.NewEngine`(또는 동급 함수)에 둔다.** 통합 테스트가 프로덕션과 똑같은 경로로 엔진을 만들 수 있어야 하기 때문이다. `main.go`는 설정 로딩, DB 연결, 서버 기동, 그레이스풀 셧다운만 담당한다.

```go
// internal/handler/router.go
func NewEngine(db sqlc.DBTX) *gin.Engine {
    userRepo := mysql.NewUserRepository(db)
    userService := service.NewUserService(userRepo)
    userHandler := NewUserHandler(userService)

    r := gin.New()
    r.Use(middleware.Recovery(), middleware.Logger())

    r.GET("/", Health)
    r.GET("/health", Health)

    v1 := r.Group("/api/v1")
    v1.GET("/users/:id", userHandler.GetUser)
    return r
}
```

### 에러 처리

- 도메인 에러는 `internal/domain/errors.go`에 `var ErrUserNotFound = errors.New("user not found")` 형태로 정의한다.
- 하위 레이어에서 올라오는 에러는 `fmt.Errorf("...: %w", err)`로 감싸 컨텍스트를 붙인다.
- HTTP 상태코드 매핑은 handler(`response.FromError`)에서만 한다. service/repository는 상태코드를 모른다.
- `panic` 사용 금지(초기화 실패 제외). 복구는 recover 미들웨어가 담당한다.

### 기본 엔드포인트

라우터에는 항상 다음 두 개가 등록되어 있어야 한다. 의존성이 없으므로 앱 조립이 살아 있는지 확인하는 스모크 테스트의 기준점이 된다.

| 메서드 | 경로 | 응답 |
|---|---|---|
| GET | `/` | `200` — `{"status":"ok"}` |
| GET | `/health` | `200` — `{"status":"ok"}` |

- 이 두 엔드포인트는 인증 미들웨어를 타지 않는다.
- DB나 외부 의존성을 건드리지 않는다. 순수하게 프로세스가 살아 있는지만 알린다.
- DB 연결까지 확인하는 체크가 필요하면 `/health/ready`를 따로 추가한다. `/health`는 바꾸지 않는다.

### API 문서

**엔드포인트 명세는 이 문서에 쓰지 않는다.** 경로, 요청 필드, 응답 코드는 핸들러의 swag 주석이 유일한 출처이고, 문서는 거기서 생성한다. 문서를 두 곳에 두면 반드시 갈라지고, 갈라진 쪽이 README다.

```bash
swag init -g cmd/api/main.go -o docs   # docs/ 생성
go run ./cmd/api                       # http://localhost:8080/swagger/index.html
```

설계 판단 — 왜 이렇게 되어 있는지 — 은 해당 핸들러의 주석에 있다. 예를 들어 가입 흐름이 왜 하나의 엔드포인트인지, 소셜 토큰 검증에서 무엇을 반드시 확인해야 하는지는 `internal/handler/auth_handler.go`의 `SignUp` 위 주석을 읽으면 된다. 이런 설명도 문서에 복제하지 않는다 — 코드를 고치는 사람이 보는 자리에 있어야 같이 갱신된다.

## 개발 규칙

### sqlc 사용 규칙

#### 워크플로

1. `migrations/`에 스키마 변경 마이그레이션을 추가한다 (sqlc가 이 파일들을 스키마로 읽는다).
2. `db/query/{entity}.sql`에 쿼리를 작성한다.
3. `sqlc generate`를 실행한다.
4. `internal/repository/mysql/{entity}_repository.go`에서 생성된 메서드를 호출하고 domain으로 매핑한다.

#### 쿼리 작성

```sql
-- db/query/user.sql

-- name: GetUser :one
SELECT id, email, name, created_at FROM users WHERE id = ?;

-- name: CreateUser :execresult
INSERT INTO users (email, name) VALUES (?, ?);

-- name: ListUsers :many
SELECT id, email, name, created_at FROM users ORDER BY id DESC LIMIT ? OFFSET ?;
```

- 쿼리 이름은 `동사 + 엔티티` (`GetUser`, `ListUsers`, `UpdateUserName`, `DeleteUser`).
- `SELECT *` 금지. 컬럼을 명시한다 — 스키마가 바뀌었을 때 생성 타입이 조용히 달라지는 걸 막는다.
- 문자열 결합으로 SQL을 만들지 않는다. 동적 필터가 필요하면 `sqlc.narg()`와 `COALESCE` 패턴으로 쿼리 안에서 처리한다.
- sqlc로 표현하기 어려운 진짜 동적 쿼리(가변 IN, 가변 정렬)만 예외적으로 repository에 손으로 작성하고, 그 이유를 주석으로 남긴다.

#### 생성 코드

- `internal/repository/mysql/sqlc/` 안의 파일은 **절대 손으로 수정하지 않는다.** 수정이 필요하면 `.sql`을 고치고 재생성한다.
- 생성 코드는 저장소에 커밋한다. 빌드 시점에 생성하지 않는다.
- 스키마나 쿼리를 바꾼 뒤에는 반드시 `sqlc generate`를 실행하고, 생성 결과까지 함께 커밋한다.
- `sqlc vet` / `sqlc diff`로 생성 코드가 최신인지 확인할 수 있다.

### Repository 작성 규칙

Repository는 sqlc 생성 코드를 감싸는 **얇은 레이어**다. 하는 일은 두 가지뿐: 쿼리 호출, 그리고 매핑(sqlc row → domain, DB 에러 → domain 에러).

```go
type UserRepository struct {
    q *sqlc.Queries
}

func NewUserRepository(db sqlc.DBTX) *UserRepository {
    return &UserRepository{q: sqlc.New(db)}
}

func (r *UserRepository) FindByID(ctx context.Context, id int64) (*domain.User, error) {
    row, err := r.q.GetUser(ctx, id)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, domain.ErrUserNotFound
        }
        return nil, fmt.Errorf("get user: %w", err)
    }
    return toDomainUser(row), nil
}
```

- 생성자는 `sqlc.DBTX`를 받는다. 프로덕션에서는 `*sql.DB`, 테스트에서는 롤백되는 `*sql.Tx`를 같은 코드에 넣기 위해서다.
- 항상 `ctx`를 전파한다.
- `sql.ErrNoRows`, 중복 키(MySQL 1062) 같은 DB 에러는 repository에서 `domain` 에러로 변환해 올린다.
- 매핑 함수(`toDomainUser`)는 같은 파일 하단에 둔다.
- 트랜잭션이 필요하면 service가 `*sql.Tx`를 열고 그 위에서 repository를 새로 만들거나 `q.WithTx(tx)`를 쓴다.

### Handler 작성 규칙

```go
func (h *UserHandler) GetUser(c *gin.Context) {
    id, err := strconv.ParseInt(c.Param("id"), 10, 64)
    if err != nil {
        response.Error(c, http.StatusBadRequest, "invalid user id")
        return
    }

    user, err := h.userService.GetUser(c.Request.Context(), id)
    if err != nil {
        response.FromError(c, err)
        return
    }
    response.OK(c, domain.NewUserResponse(user))
}
```

- 라우트 등록은 `handler/router.go` 한 곳에서만 한다.
- 요청 바인딩은 `ShouldBindJSON` + `binding` 태그로 검증한다.
- 응답은 `internal/pkg/response` 헬퍼를 통해서만 반환한다. `c.JSON` 직접 호출 금지.
- domain 엔티티를 그대로 응답하지 않는다. 응답 DTO로 변환한다.

### 설정

- 모든 설정은 `internal/config`에서 환경변수로 로딩한다. 하드코딩 금지.
- 시크릿을 저장소에 커밋하지 않는다. `.env`는 gitignore.
- 필수 환경변수가 없으면 부팅 시점에 즉시 실패시킨다.

### 네이밍

- 파일: `snake_case.go` (`user_handler.go`, `user_repository.go`)
- 패키지: 소문자 단수형, 언더스코어 없음 (`handler`, `service`, `mysql`)
- 생성자: `New{Type}` — 항상 포인터 반환
- 리시버: 1~2글자 (`func (r *UserRepository)`, `func (s *UserService)`)

### 새 기능 추가 절차

`{Entity}` 하나를 추가할 때 순서:

1. `migrations/` — 테이블 마이그레이션
2. `db/query/{entity}.sql` — 쿼리 작성
3. `sqlc generate` — 생성 코드 갱신
4. `internal/domain/{entity}.go` — 엔티티, 요청/응답 DTO, 도메인 에러
5. `internal/repository/mysql/{entity}_repository.go` — 생성 코드 호출 + domain 매핑
6. `internal/service/{entity}_service.go` — repository 구체 타입을 필드로 받는 서비스
7. `internal/handler/{entity}_handler.go` — 핸들러
8. `internal/handler/router.go` — 조립 + 라우트 등록
9. `test/integration/{entity}_test.go` — 성공 경로 + 실패 경로 통합 테스트

## 마이그레이션

### 파일명

```
{배포순번4자리}{시퀀스2자리}_{티켓ID}_{설명}.up.sql
{배포순번4자리}{시퀀스2자리}_{티켓ID}_{설명}.down.sql
```

```
migrations/
├── 000101_A1_create_customer.up.sql
├── 000101_A1_create_customer.down.sql
├── 000102_A1_create_customer_social.up.sql
└── 000102_A1_create_customer_social.down.sql
```

같은 티켓에서 나온 작업은 배포순번과 티켓 ID가 같아 한 배포 단위임이 파일 목록에서 바로 보인다.

**version은 파일명 앞 숫자에서 선행 0을 제거한 정수다.** `000101` → `101`, `000102` → `102`. 아래 명령에 쓰는 숫자는 이 값이다.

티켓 ID를 version이 아니라 이름 자리에 두는 이유는 golang-migrate의 version이 부호 없는 64비트 정수여야 하기 때문이다. `A1` 같은 영문 포함 티켓은 prefix가 될 수 없다.

### 명령어

아래 명령은 모두 `source .env`로 `DB_DSN`을 읽은 상태를 전제한다.

```zsh
export MIGRATE="migrate -path migrations -database mysql://$DB_DSN"
```

| 목적 | 명령 |
|---|---|
| 현재 버전 확인 | `$MIGRATE version` |
| 전부 적용 | `$MIGRATE up` |
| **특정 버전까지 적용** | `$MIGRATE goto 104` |
| 특정 버전으로 되돌리기 | `$MIGRATE goto 102` |
| 전부 되돌리기 | `$MIGRATE down -all` |

#### 특정 티켓 범위만 적용하기

102까지 적용된 상태에서 103, 104를 올리는 경우:

```zsh
$MIGRATE version   # 102 확인
$MIGRATE goto 104
$MIGRATE version   # 104 확인
```

되돌릴 때는 `$MIGRATE goto 102`.

**`up N` / `down N`보다 `goto`를 쓴다.** `goto`는 목표 지점이 절대적이라 몇 번을 실행해도 결과가 같다. `up 2`는 "지금부터 2개"라서 현재 버전을 잘못 알고 있으면 엉뚱한 곳에서 멈추고도 성공한 것처럼 끝난다.

**골라서 적용할 수는 없다.** golang-migrate는 순차적이라 `goto 104`는 "104번까지 전부"라는 뜻이지 "104번만"이 아니다. 이미 105가 적용된 상태에서 뒤늦게 103을 끼워 넣는 것도 불가능하다. 그래서 배포순번은 티켓을 만든 순서가 아니라 **실제 배포 순서**로 매긴다.

#### 실패해서 dirty 상태가 됐을 때

103은 성공했는데 104에서 SQL 오류가 나면 DB가 dirty로 표시되고, 이후 모든 명령이 거부된다:

```
Dirty database version 104. Fix and force version.
```

MySQL은 DDL이 트랜잭션으로 묶이지 않으므로 **104가 절반만 적용됐을 수 있다.** 순서는 이렇다.

1. 104의 SQL을 보고 DB에 무엇이 실제로 반영됐는지 확인한다
2. 수동으로 정리해서 DB를 103 시점 상태로 되돌린다
3. 버전 기록을 실제 상태에 맞춘다

```zsh
$MIGRATE force 103
```

`force`는 마이그레이션을 실행하지 않고 버전 기록만 바꾼다. **DB 실제 상태를 확인하고 일치시킨 뒤에만 쓴다.** 확인 없이 쓰면 스키마와 기록이 어긋난 채로 다음 마이그레이션이 올라간다.

## 로컬 DB

로컬 개발용 MySQL은 `docker-compose.yml`로 띄운다. 고정 포트 `127.0.0.1:3306`이라 GUI 클라이언트가 항상 같은 주소로 붙는다.

```bash
docker compose up -d --wait      # 기동 (healthcheck 통과까지 대기)
docker compose stop              # 정지 (데이터 유지)
docker compose down -v           # 볼륨까지 삭제 = 완전 초기화
```

- 기동 후 스키마는 `migrate`로 올린다. compose가 마이그레이션을 대신 실행하지 않는다.
- **이 컨테이너는 통합 테스트와 무관하다.** 테스트는 testcontainers가 매번 새 MySQL을 랜덤 포트에 띄우고 끝나면 버린다. 로컬 DB의 데이터가 테스트 결과에 영향을 주지 않고, 그 반대도 마찬가지다.
- 포트는 루프백(`127.0.0.1`)에만 바인딩한다. `0.0.0.0`으로 열지 않는다.
- 접속 정보는 `.env`의 `DB_DSN` 하나로 관리한다. 코드나 다른 파일에 DSN을 중복해 적지 않는다.

## 테스트

**단위 테스트를 위한 mock을 만들지 않는다.** 이 프로젝트의 기본 테스트는 실제 MySQL 컨테이너를 띄우고 실제 HTTP 요청을 보내는 통합 테스트다. 순수 함수(계산, 검증, 포맷 변환)만 mock 없는 단위 테스트로 작성한다.

### 테스트 계층

| 종류 | 위치 | 범위 |
|---|---|---|
| 통합(API) | `test/integration/` | HTTP 요청 → 라우터 → 미들웨어 → handler → service → 실제 DB → 응답 |
| 리포지토리 | `internal/repository/mysql/*_test.go` | 실제 DB에 대한 쿼리 동작, 제약조건, 에러 매핑 |
| 단위 | 해당 패키지 옆 | 외부 의존성 없는 순수 로직만 |

기본은 통합 테스트다. 새 엔드포인트를 추가하면 `test/integration/`에 테스트를 추가한다.

sqlc가 컴파일 타임에 컬럼/타입 오류를 잡아주지만, 그건 쿼리가 **컴파일된다**는 뜻이지 **의도대로 동작한다**는 뜻이 아니다. JOIN 결과, 정렬, 페이징, 제약조건 위반, 트랜잭션 경계는 여전히 실제 DB로 검증해야 한다.

### DB: testcontainers + 실제 마이그레이션

- 테스트 DB는 `testcontainers-go`로 실제 MySQL 컨테이너를 기동한다. sqlite 대체 금지 — 프로덕션과 다른 SQL 방언에서 검증하면 의미가 없다.
- 스키마는 `migrations/`를 그대로 실행해서 만든다. 테스트 전용 스키마 DDL을 따로 두지 않는다. 마이그레이션이 깨지면 테스트도 깨져야 한다.
- 컨테이너는 패키지당 한 번만 띄운다(`TestMain`). 테스트마다 재기동하지 않는다.

```go
// internal/testutil/db.go
var testDB *sql.DB

func MustStartDB(m *testing.M) int {
    ctx := context.Background()
    c, err := mysqlctr.Run(ctx, "mysql:8.0", mysqlctr.WithDatabase("app_test"))
    if err != nil {
        log.Fatalf("start mysql: %v", err)
    }
    defer c.Terminate(ctx)

    dsn, _ := c.ConnectionString(ctx, "parseTime=true", "multiStatements=true")
    testDB = sql.OpenDB(...)
    MustMigrateUp(testDB) // migrations/ 디렉터리를 그대로 적용

    return m.Run()
}

// test/integration/main_test.go
func TestMain(m *testing.M) { os.Exit(testutil.MustStartDB(m)) }
```

### 테스트 간 격리

각 테스트는 자기 데이터만 보고, 끝나면 흔적을 남기지 않는다. 다음 중 하나를 쓴다:

1. **트랜잭션 롤백** (기본) — 테스트 시작 시 트랜잭션을 열고 `t.Cleanup`에서 롤백한다. 가장 빠르다.
2. **테이블 truncate** — 트랜잭션 동작 자체를 검증하는 테스트처럼 롤백을 쓸 수 없는 경우에만.

```go
func NewTx(t *testing.T) *sql.Tx {
    t.Helper()
    tx, err := testDB.BeginTx(context.Background(), nil)
    require.NoError(t, err)
    t.Cleanup(func() { _ = tx.Rollback() })
    return tx
}
```

`*sql.Tx`는 `sqlc.DBTX`를 만족하므로 `handler.NewEngine(tx)`에 그대로 넣을 수 있다. 이게 repository 생성자가 `DBTX`를 받는 이유다.

- 테스트끼리 공유하는 전역 시드 데이터를 만들지 않는다. 필요한 데이터는 각 테스트가 픽스처로 직접 만든다.
- 테스트 실행 순서에 의존하지 않는다. 순서를 바꿔도 통과해야 한다.
- 고정 ID를 하드코딩하지 않는다. 픽스처가 만든 값을 받아 쓴다.

### 앱 전체를 조립해서 테스트한다

통합 테스트는 handler를 직접 호출하지 않는다. 프로덕션과 **동일한 조립 함수**(`handler.NewEngine`)로 엔진을 만들고 `httptest`로 실제 요청을 보낸다. 미들웨어, 라우팅, 바인딩, 상태코드, JSON 응답까지 함께 검증하기 위함이다.

```go
// test/integration/user_test.go
func TestCreateUser(t *testing.T) {
    app := testutil.NewApp(t, testutil.NewTx(t))

    res := app.POST("/api/v1/users", `{"email":"a@b.com","name":"remi"}`)

    require.Equal(t, http.StatusCreated, res.Code)
    var body domain.UserResponse
    require.NoError(t, json.Unmarshal(res.Body.Bytes(), &body))
    require.Equal(t, "a@b.com", body.Email)
}
```

### 무엇을 검증하는가

- 상태코드와 응답 body의 실제 JSON 필드 (구조체 필드가 아니라 직렬화 결과)
- 요청 후 **DB에 실제로 반영된 값** — service 반환값만 믿지 않는다
- 실패 경로: 검증 실패(400), 인증 실패(401), 없음(404), 중복 키 같은 제약조건 위반(409)
- 인증이 필요한 엔드포인트는 토큰 없이/잘못된 토큰으로도 호출해 본다

### 외부 의존성

- DB, Redis 등 컨테이너로 띄울 수 있는 것은 실제로 띄운다.
- 외부 HTTP API만 `httptest.Server`로 가짜 서버를 세운다. 이때도 인터페이스를 만들지 않고 base URL을 설정으로 주입해 바꾼다.
- 테스트가 인터넷에 나가지 않는다.

### 규칙

- 테스트는 `require`(실패 시 중단)를 기본으로 쓴다.
- `time.Sleep`으로 기다리지 않는다. 조건을 폴링하거나 동기화한다.
- 컨테이너 기동이 필요한 테스트는 `testing.Short()`일 때 건너뛴다(`go test -short`로 순수 단위 테스트만 실행 가능).
- 테스트 이름은 `Test{대상}_{상황}_{기대}` 형태로 쓴다. 예: `TestCreateUser_DuplicateEmail_Returns409`

## 린트

`golangci-lint`가 CI의 게이트다. 설정은 `.golangci.yml`에 있고, 이 문서의 레이어 규칙 중 기계로 검사 가능한 것은 `depguard`로 강제된다:

| 규칙 | 검사 내용 |
|---|---|
| `sqlc-stays-in-repository` | service/handler/domain/middleware에서 sqlc 생성 패키지 import 금지 |
| `http-stays-in-handler` | service/repository에서 gin import 금지 |
| `domain-has-no-deps` | domain이 다른 internal 패키지 import 금지 |
| `no-upward-import` | repository → service/handler 역방향 import 금지 |

- 린트 위반을 `//nolint`로 덮지 않는다. 코드를 고친다. 불가피하면 반드시 사유를 함께 적는다: `//nolint:gosec // 이유`
- 새 레이어 규칙이 생기면 문서에만 적지 말고 `.golangci.yml`의 depguard에도 추가한다.
- sqlc 생성 코드(`internal/repository/mysql/sqlc/`)는 린트 대상에서 제외돼 있다.
- 포맷은 `golangci-lint fmt`(gofmt + goimports, local prefix `golang-gin`)로 통일한다.

## CI

`.github/workflows/ci.yml`이 PR마다 세 개의 잡을 돌린다:

- `lint` — golangci-lint + gofmt 확인
- `test` — `go build` + `go test -race`. ubuntu 러너에 Docker가 있으므로 testcontainers 통합 테스트도 함께 실행된다
- `sqlc` — `sqlc diff`로 생성 코드가 `.sql`과 어긋나지 않았는지 확인

CI를 통과시키려고 테스트를 skip 처리하거나 린터를 비활성화하지 않는다.

## 개발 환경 (VS Code)

- 디버그 구성은 `.vscode/launch.json`에 있다. 새 실행 시나리오가 필요하면 여기에 추가한다.
  - `API 서버 디버그` — `cmd/api`를 delve로 기동. `.env`를 읽는다.
  - `현재 파일의 테스트 디버그` / `커서 위치의 테스트 하나만 디버그` — 열려 있는 파일 기준. `.env.test`를 읽는다.
  - `통합 테스트 전체 디버그` — `test/integration` 패키지 전체. Docker 필요.
  - `실행 중인 프로세스에 붙기` / `원격 delve에 붙기` — 이미 떠 있는 프로세스 디버깅용.
- 환경변수는 `.env`(로컬 실행) / `.env.test`(테스트)에 둔다. 둘 다 gitignore 대상이고, 키가 추가되면 `.env.example`·`.env.test.example`도 같이 갱신한다.
- 컨테이너를 띄우는 테스트는 브레이크포인트에서 멈춰 있는 동안 타임아웃이 나지 않도록 `-test.timeout`을 넉넉히 준다(launch.json에 이미 설정됨).
- 저장 시 `goimports` 포맷 + import 정리가 자동 적용된다. 포맷 때문에 별도 커밋을 만들지 않는다.
- 태스크(`Cmd+Shift+B` / 테스트 실행)는 `.vscode/tasks.json`에 있다: build, vet, test(short/all), `sqlc generate`, `migrate up/down`.

## 명령어

```bash
go run ./cmd/api          # 실행
go build ./...            # 빌드
sqlc generate             # 쿼리 → Go 코드 생성
swag init -g cmd/api/main.go -o docs   # API 문서 생성
sqlc vet                  # 쿼리 검증
go test ./...             # 전체 테스트 (Docker 필요)
go test -short ./...      # 컨테이너 없는 테스트만
go vet ./...              # 정적 분석
golangci-lint run         # 린트 (레이어 규칙 검사 포함)
golangci-lint fmt         # 포맷 자동 적용
gofmt -l .                # 포맷 확인
```
