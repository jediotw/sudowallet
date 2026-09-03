docker compose down -v: The basic docker-compose down command removes containers and networks but keeps volumes.
 Use --volumes to remove volumes when you want to reset your data. 
 Use --rmi to remove images when you need to free up disk space.

 # omitempty is mainly used when serializing a struct to JSON so empty fields are omitted from the output. It doesn't help for required input validation like DTO
# the idiomatic Go style.

type mysqlUserRepository struct {
    db *sql.DB
}

func NewMySQLUserRepository(db *sql.DB) UserRepository {
    return &mysqlUserRepository{db: db}
}

Here's why it works:

mysqlUserRepository (lowercase m) is unexported.
It can only be used inside the same package.
Other packages cannot create it directly.
NewMySQLUserRepository (uppercase N, M, SQL, U, R) is exported.
Other packages can call it.
It acts as the public constructor.
From another package
repo := repository.NewMySQLUserRepository(db)

This works.


# errors: Different developers Different errors Inconsistent API responses Clients become harder to build Define one error contract Create AppError Centralize error handling
When building an API How do I want my API to communicate failures?
Every error needs:
    ├── HTTP status   → what happened at HTTP level?
    ├── error code    → what happened at application level?
    └── message       → human-readable explanation

so use struct
    type AppError struct {
    StatusCode int
    Code       string
    Message    string
}
Then:
"I need this to work with Go's normal error handling."

Go's error interface is:

type error interface {
    Error() string
}

Therefore:

func (e *AppError) Error() string {
    return e.Message
}

Then:

"I don't want to manually construct this everywhere."

So:

func NewAppError(status int, code, message string) *AppError {
    return &AppError{
        StatusCode: status,
        Code:       code,
        Message:    message,
    }
}

Then:

"What errors does my application commonly have?"

500 → INTERNAL_SERVER_ERROR
400 → BAD_REQUEST
404 → NOT_FOUND
401 → UNAUTHORIZED

And you get:

var (
    ErrInternalServer = NewAppError(500, "INTERNAL_SERVER_ERROR", "...")
    ErrBadRequest     = NewAppError(400, "BAD_REQUEST", "...")
    ErrNotFound       = NewAppError(404, "NOT_FOUND", "...")
    ErrUnauthorized   = NewAppError(401, "UNAUTHORIZED", "...")
)

That's it.

Formula:
API error
   ↓
Status + Code + Message
   ↓
Must satisfy Go error interface
   ↓
Constructor prevents repetitive creation
   ↓
Standard errors prevent inconsistent API responses
# why do we need centralized logger because if we have 100 instances of backend with individual log then we would be end up with 100 ssh for each machine so we make centrealized logger and every request goes through it \.
## what we want from our system
1. fucntional
i. get the information of the request from all the instances of the application
ii.support info,debug,warn,error
iii.search log capability
iv.fiter by service,time,request_id
v. can help in corelate logs across all the services
vi.support structured log
v.retain logs for longer period
2. non-functional
i.high performance
ii.low impact on application performance
iii.high availibility
iv.faster query in stored logs

## suppose one log goes through lots of services and we want a request id on evry log then we can search request id and u will get the service it been through and and actions
Then we can search:

request_id = abc123

and get:

API Gateway      request received
User Service     user authenticated
Order Service    order created
Payment Service  payment started
Payment Service  payment timeout
Order Service    order failed this is request corealation.

# why not send the log directly to the database?
suppose imagine 10k req/sec  and each req produces 10 log enteries that's 100k log enteries/sec and every application instance write sychronously to the logging db then response then this is a critical path moreover if elastic search becomes slow then your application is slow as appication waits db waits and logging is slow.
so make logging asychronous so need not to wait for the entire pipeline this gives us decoupling.
why do we need kafka as buffer 
suppose 10k application insatances -> log collector->kafka->consumers
suppose we produced 50mb/s log but our storage can only process 10mb/s and without buffer storage will be overwhelmed w/ kafka it gives us buffering,decoupling,replay,horizontal scaling,fault tolerance 
# where should collection happen ?
1. there are two approaches :application direclty send logs e.g application->logginga api->kafka this give us centralized control but evry application need networking logic for logging.
2. agent based collection (preferred)
appication->stdout/stderr( streams (often captured in files on disk by container engines like Docker or Kubernetes))->log agent( parse,collect, filter, transform, and ship them to databases or cloud storage) ->kafka
the collector handles batching,retries,buffering,compression,forwarding,metadata enrichment

then comes the storage design:
We want a system optimized for:

huge writes
time-based data
text/field search
retention
A common architecture is something like:

Kafka
  │
  ▼
Log Processor
  │
  ▼
OpenSearch / Elasticsearch for searchable recent logs.
But separate hot and cold storage.
                    Logs
                      │
                      ▼
                    Kafka
                      │
              ┌───────┴────────┐
              ▼                ▼
       Hot Storage        Object Storage
       7–30 days          30–365+ days
              │                │
              ▼                ▼
          Fast Search        Cheap Archive
## Scaling the system
We need horizontal scaling.
Let's say we have:1 TB logs/day
Eventually:10 TB/day or 100 TB/day
 collection from all the agent were going to one kafka 
 Kafka partitions allow consumers to process logs concurrently.
 now processing becomes  kafka with n consumers.

 shard index based on time,region,services,
# Q. if kafka is down and dont want my appliction to be down so we use local logger with local buffer s kafka is back local buffer to kafka but cannot have infinite local buffer so buffer full we drop the debug log first,then  info log but always keep warn/error log .

# massive security surface our centralized logging so we need to take care of RBAC,redaction,retention period of different log level
## log processor 
the log processor receive log from kafka then it can validate, enrich,readact(black out/prevent) the sensitive info from reaching centralized log,
# logger:A unique identifier attached to one request so you can connect all logs belonging to that request.
because there are 100s of request coming in a second and we need to identify those req and response and errors and problems
Request A
correlation_id = abc123

Request B
correlation_id = xyz789

Logs:

{
  "level": "INFO",
  "msg": "request received",
  "correlation_id": "abc123"
}
{
  "level": "ERROR",
  "msg": "database failed",
  "correlation_id": "abc123"
}
now we can search the request with correaltional id and reconstruct the request journey

we have context which carries request id so it tells Which log belongs to which request?

We need some identity for a request.

so we need a centrealized logger
we need consistent logging.
Every request should have an identity that can follow it through the system.

# remember this what u need in ur log file
slog.Logger
      ↓
slog.Handler
      ↓
HandlerOptions
      ├── Level      → which logs?
      └── AddSource  → where did log come from?

# slog.HandlerOptions has exactly 3 fields.

type HandlerOptions struct {
    AddSource   bool
    Level       Leveler
    ReplaceAttr func(groups []string, a Attr) Attr
}

Let's understand each one.

1. Level
Level: slog.LevelInfo,

This controls the minimum severity that gets logged.

slog levels are roughly:The levels have an ordering
TRACE
DEBUG
INFO -> and we have configured our handler to have minimum level to be info
WARN
ERROR
FATAL
 we can configure level enable or disable if dubuging is disabled for payment service then during debugging we will see dubugging is disabled for payment
If you configure:

Level: slog.LevelInfo,

then:

DEBUG  → ❌ ignored
INFO   → ✅
WARN   → ✅
ERROR  → ✅

If:

Level: slog.LevelDebug,

then everything from DEBUG upward is allowed.

The default is INFO.

2. AddSource
AddSource: true,

This tells slog:

"Include the source location of the log statement."

For example:

logger.Error("database failed")

Without AddSource:

{
  "level": "ERROR",
  "msg": "database failed"
}

With:

AddSource: true,

you get additional information such as:

{
  "level": "ERROR",
  "msg": "database failed",
  "source": {
    "file": "internal/user/service.go",
    "line": 42
  }
}

So this is useful when debugging: which file and line generated this log?

3. ReplaceAttr

This is the advanced one.

ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
    // modify the attribute
    return a
},

It lets you modify, rename, sanitize, or remove log attributes before they're output.

For example, suppose you have:

logger.Info(
    "user login",
    "email", "saurabh@example.com",
)

You don't want the email appearing in your logs.

You can use ReplaceAttr to remove it.

Conceptually:

Logger
  ↓
"email" = "saurabh@example.com"
  ↓
ReplaceAttr
  ↓
REMOVE email
  ↓
JSON Handler

It can also modify built-in fields like:

time
level
source
msg

and normal attributes.

# Log variable which of * slog.Logger type is intially pointing to nil but when i created the object of slogger it is a reference to one slog.Logger object.
Log = slog.New(handler)
viz,Create an slog.Logger configured with this handler, and store a pointer to it in Log.
Log
 │
 ▼
┌──────────────────┐
│ slog.Logger      │
│                  │
│ handler          │
│ level config     │
│ logging methods  │
└──────────────────┘

// a helper function that automatically  takes the correlation ID OUT of the context and adds it to the log arguments.
// Because slog logging arguments can contain different types like
logger.Info(
    "user created",
    "user_id", 123,              // int
    "email", "test@example.com", // string
    "premium", true,             // bool
)
a slice that can contain values of different types.-> []any
only any==interface
so,Give me the request context and the logging arguments; I'll potentially add the correlation ID and give you the updated logging arguments back.
func getLogArgs(ctx context.Context, args []any) []any

# if you know your application stores the correlation ID as a string, you need to extract it as a string.

The normal Go way is:

cid, ok := ctx.Value(CorrelationIDKey).(string)

This is called a type assertion.
or 
 u can create a util function 

# ... in function method is unpacking 


# context
                    Context
                       │
        ┌──────────────┼──────────────┐
        │              │              │
     Values        Cancellation    Deadline
        │              │              │
 correlation_id     Done()        Timeout
 user_id            Err()         Deadline

 And the main constructors are:
 Background()
     │
     ├── WithValue()
     │      └── attach request data
     │
     ├── WithCancel()
     │      └── manually stop work
     │
     ├── WithTimeout()
     │      └── stop after duration
     │
     └── WithDeadline()
            └── stop at specific time

WithValue() → put request-scoped data into context

Value() → retrieve that request-scoped data from context


# type assertion
cid, ok := ctx.Value(CorrelationIDKey).(string)
Because context.Value() returns:any
So Go doesn't know what type is inside it.

You are saying:"I expect this to contain a string" so, .(string)
eg2:if appErr, ok := err.(*customErr.AppError)
here err.(*customErr.AppErr) is a type assertion mean
							"What is actually inside err?"
					                 │
					                 ▼
					       Is it *customErr.AppErr?
					                 │
					          ┌──────┴──────┐
					          │             │
					         YES            NO
					          │             │
					          ▼             ▼
					       return        assertion
					      *AppErr        fails
				customErr is alias for package and AppErr The AppErr type defined in that package. so *customErr.AppErr a pointer to an AppErr.


# jwt claims

JWT (JSON Web Token) claims are pieces of information stored inside a JWT. They describe who the user is, what they're allowed to do, or other metadata about the token.

A JWT has three parts:

Header.Payload.Signature

The payload contains the claims.

For example:

{
  "sub": "1234567890",
  "name": "Alice",
  "email": "alice@example.com",
  "role": "admin",
  "iat": 1692374400,
  "exp": 1692378000
}

Each key-value pair is a claim.

Types of JWT Claims
1. Registered Claims (Standard)

These are predefined by the JWT specification.

Claim	Meaning
iss	Issuer (who created the token)
sub	Subject (usually the user ID)
aud	Audience (who the token is intended for)
exp	Expiration time
nbf	Not before (token isn't valid before this time)
iat	Issued at
jti	JWT ID (unique token identifier)

Example:

{
  "iss": "https://auth.example.com",
  "sub": "user123",
  "aud": "my-api",
  "exp": 1712345678,
  "iat": 1712342078
}
2. Public Claims

These are custom claims that are intended to be shared publicly and should avoid naming conflicts.

Example:

{
  "role": "admin",
  "department": "engineering"
}
3. Private Claims

These are application-specific claims agreed upon between the issuer and the consumer.

Example:

{
  "tenantId": "company-42",
  "permissions": [
    "read:orders",
    "write:orders"
  ]
}
How Claims Are Used

Imagine a user logs into an application.

User authenticates.
Authentication server creates a JWT.
Claims are added:
{
  "sub": "42",
  "name": "Alice",
  "role": "admin",
  "exp": 1712345678
}
Client sends the JWT with each request:
Authorization: Bearer <jwt>
The API verifies the signature and reads the claims.

Then it can do things like:

role == "admin" → allow deleting users
role == "user"  → deny deleting users
Example in Code

Suppose the payload is:

{
  "sub": "123",
  "email": "alice@example.com",
  "role": "admin",
  "exp": 1712345678
}

After decoding the JWT in your application, you might access the claims like this:

const claims = decodedJwt;


console.log(claims.sub);    // "123"
console.log(claims.email);  // "alice@example.com"
console.log(claims.role);   // "admin"


# The JSON tag determines the JSON key so in go field whatever name u want u can keep but in serilizatioon(json tag) u have to mention the actual name inorder to match: ref jwt claims
1. Generate/Create a JWT
Use case: User successfully logs in

You need to create an access token.

token := jwt.NewWithClaims(
    jwt.SigningMethodHS256,
    claims,
)

Then sign it:

signedToken, err := token.SignedString(secret)

So the flow is:

Login
  ↓
Create claims
  ↓
jwt.NewWithClaims()
  ↓
SignedString()
  ↓
JWT string

Example:

claims := Claims{
    RegisteredClaims: jwt.RegisteredClaims{
        ID:        uuid.New().String(),
        Subject:   userID,
        IssuedAt:  jwt.NewNumericDate(time.Now()),
        ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
    },
    Role: "user",
}


token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)


signedToken, err := token.SignedString(secret)
2. Parse/Validate a JWT
Use case: API receives a request

Client sends:

Authorization: Bearer eyJhbGci...

Your middleware needs to verify it.

That's where:

jwt.Parse()

or:

jwt.ParseWithClaims()

comes in.

For structured claims, you'll commonly use:

token, err := jwt.ParseWithClaims(
    tokenString,
    &claims,
    keyFunc,
)

The flow:

HTTP Request
     ↓
Extract JWT
     ↓
ParseWithClaims()
     ↓
Verify signature
     ↓
Validate claims
     ↓
Get user identity
3. ParseWithClaims() — important

If you created custom claims:

type Claims struct {
    jwt.RegisteredClaims
    Role string `json:"role"`
}

then when parsing:

var claims Claims


token, err := jwt.ParseWithClaims(
    tokenString,
    &claims,
    func(token *jwt.Token) (any, error) {
        return secret, nil
    },
)

After successful parsing:

claims.Subject
claims.Role
claims.ID

give you:

claims.Subject → sub
claims.Role    → role
claims.ID      → jti
4. jwt.NewWithClaims()
Use case: Creating a token with your payload

Remember:

jwt.New()

creates a token without your claims.

jwt.NewWithClaims()

creates a token with claims.

So for authentication, you'll usually want:

jwt.NewWithClaims(...)
5. SignedString()
Use case: Turn the token into the actual JWT string

Before:

token := jwt.NewWithClaims(...)

You have a Go object.

After:

signedToken, err := token.SignedString(secret)

you get:

eyJhbGciOiJIUzI1NiIs...

That's the thing you send to the client.

6. jwt.RegisteredClaims

This isn't a function.

It's a struct containing standard JWT claims.

type Claims struct {
    jwt.RegisteredClaims


    Role string `json:"role"`
}

Use it whenever you want standard claims such as:

jti
sub
iss
aud
exp
nbf
iat
7. jwt.NewNumericDate()
Use case: Creating exp, iat, etc.

Instead of manually calculating Unix timestamps:

ExpiresAt: jwt.NewNumericDate(
    time.Now().Add(15 * time.Minute),
)

Example:

claims := jwt.RegisteredClaims{
    IssuedAt: jwt.NewNumericDate(time.Now()),


    ExpiresAt: jwt.NewNumericDate(
        time.Now().Add(15 * time.Minute),
    ),
}
8. token.Valid
Use case: After parsing, check whether the token is valid

You might see:

if err != nil || !token.Valid {
    // reject request
}

Conceptually:

Parsing succeeded?
       ↓
Signature valid?
       ↓
Claims valid?
       ↓
token.Valid == true
       ↓
Allow request
9. jwt.WithValidMethods()

This is an important security-related option.

When parsing a token, don't blindly accept whatever signing algorithm the token claims to use.

You can restrict it:

jwt.ParseWithClaims(
    tokenString,
    &claims,
    keyFunc,
    jwt.WithValidMethods([]string{"HS256"}),
)

Meaning:

"I only accept HS256 tokens."

This is a good production habit.


# how validation of jwt works?

// client username +password -> server -> jwt token string(access token)
// now client send the jwt payload  back to the server on the protected request, server will verify the token and if it is valid, it will allow the request to proceed. If the token is invalid, the server will return an error response.
// When your server validates the JWT, it creates an empty Go claims structure
// then parsewithclaim decodes the jwt payload and populates the claims structure with the data from the token. The server can then access the claims data and use it to authorize the request.

             JWT
              │
              ↓
      ParseWithClaims()
              │
       ┌──────┴───────┐
       ↓              ↓
   Decode claims   Verify signature
       │              │
       └──────┬───────┘
              ↓
         Validate claims
              ↓
          token.Valid
              ↓
           ACCEPT

this "func(token *jwt.Token)" tell jwt libary "Which key should I use to verify the signature of this JWT?"
JWT itself contains information about the signing algorithm in its header.
here token is a Go *jwt.Token object.It contains things like:
Header
Claims
Signing method
after singing the token it becomes singedJWT string this is what client sends
during validation The client sends the signed JWT string back
u extract it and pass it to the parsewithclaims
The library then parses that signed string into a *jwt.Token and verifies its signature. ref: auth.go for more  


# gin has several binding methods 
| Method             | JSON?   | Content-Type       | On error                      |
| ------------------ | ------- | ------------------ | ----------------------------- |
| `ShouldBindJSON()` | ✅       | JSON only          | Returns error                 |
| `BindJSON()`       | ✅       | JSON only          | Automatically aborts with 400 |
| `ShouldBind()`     | Depends | Auto-detects       | Returns error                 |
| `MustBindWith()`   | Depends | You specify binder | Automatically aborts with 400 |

# Common gin.Context MethodsIf you meant standard request/response methods on a Gin c variable (*gin.Context), you normally use:
Request parsing: c.Param(), c.Query(), or c.ShouldBindJSON()
Response writing: c.JSON(), c.String(), or c.HTML()
Data passing: c.Set() and c.Get()
Flow control: c.Abort() or c.Next()


# transaction 
1. prepare query 
2. if this query is update update/insert/delete then use ExecuteContext method of the transaction object which takes context, query and the parameters to be inserted into the query.

##Remember
SELECT single row → tx.QueryRowContext()
SELECT multiple  → tx.QueryContext()
INSERT/UPDATE/DELETE → tx.ExecContext()

# BACKTICK VS DOUBLE QUOTE
In Go, SQL queries are just strings. You commonly see two forms:
query := "SELECT id, balance FROM wallets WHERE id = ?"
If you need to write the query across multiple lines, you have to use \n and that's annoying for sql

query := `
	SELECT id, balance
	FROM wallets
	WHERE id = ?
`
raw string literal in Go.
it allows:

Multiple lines
Quotes without escaping
Cleaner SQL formatting

-Go converts both into a string before sending the query to the database.

# why Error.Is in wallet repository getbyuserid?
QueryRowContext() is designed to represent one expected row.
but Scan() return sql.ErrNoRows?
so error is a go standard library 
so error.Is()
Check whether an error is/contains that specific error

Scan()
  ↓
err
  ↓
Is err != nil?
  │
  ├── NO ─────────────→ continue normally
  │
  └── YES
       ↓
    Is it sql.ErrNoRows?
       │
       ├── YES ───────→ return nil, ErrWalletNotFound
       │
       └── NO ────────→ return nil, err


# if your service is responsible for an operation that involves both User and Wallet, then the service needs access to both repositories.
So,type UserService struct {
    userRepo   repository.UserRepository
    walletRepo repository.WalletRepository
    db         *sql.DB
}
this is called dependency composition
if user service is dependent upon user repository and wallet repository then we can use the interface of user repository and wallet repository in the user service and like wise we call is dependency composition. This is called implicit interface implementation. The user service does not need to know the concrete implementation of the user repository and wallet repository, it just needs to know the interface. This allows us to easily swap out the implementation of the user repository and wallet repository without changing the user service. This is a good practice in software design as it promotes loose coupling and high cohesion.

# in service layer
//since the responsibiity of this register function is to create a user and a wallet for that user, we can use the transaction to ensure that both the user and the wallet are created successfully or none of them are created. This is called atomicity. If the user creation fails, the wallet creation will not be attempted and vice versa. This ensures that the database remains in a consistent state.


# Sentinel errors and NewAppError() solve different problems.

Sentinel error

You define one fixed error:

var ErrUserNotFound = errors.New("user not found")

Then you return the same error value:

return nil, ErrUserNotFound

Later:

if errors.Is(err, ErrUserNotFound) {
    // handle user not found
}

This is useful when you only need to identify what kind of error occurred.

NewAppError()

Your NewAppError() seems to carry HTTP/API information:

customErr.NewAppError(
    http.StatusConflict,
    "EMAIL_ALREADY_REGISTERED",
    "Email is already registered.",
)

It gives you more information:

status = 409
code   = EMAIL_ALREADY_REGISTERED
message = Email is already registered.

So you use it when you want a specific API response.

Use a sentinel when you need identity
var ErrUserNotFound = errors.New("user not found")

Then:

errors.Is(err, ErrUserNotFound)
Use AppError when you need HTTP/API metadata
return nil, NewAppError(
    http.StatusConflict,
    "EMAIL_ALREADY_REGISTERED",
    "Email is already registered.",
)

//now dont need to create user using create method of user repository because we have already created the user using the transaction. So we can return the user object directly. But we need to fetch the user from the database to return the stored record. This is because the user object we have created is not the same as the one stored in the database. The database may have added some fields like created_at, updated_at, etc. So we need to fetch the user from the database to return the stored record.
	if err := s.userRepo.Create(ctx, user); err != nil {
		//return a internal server error if user creation fails but why not custom error? because this is an unexpected error and we dont want to expose the internal server error to the client
		return nil, customErr.NewAppError(http.StatusInternalServerError, "REGISTRATION_FAILED", "Failed to complete registration.")
	}
  //no need to fetch user from db and then jsut return the creted user before trxn.

# how to set to context and get from context
  | Where the data comes from | Gin method                  | Remember               |
| ------------------------- | --------------------------- | ---------------------- |
| Middleware stored it      | `c.Get()` / `c.GetString()` | **Set → Get**          |
| URL path                  | `c.Param()`                 | **Path → Param**       |
| Query string              | `c.Query()`                 | **? → Query**          |
| JSON body                 | `c.ShouldBindJSON()`        | **JSON → Bind**        |
| Form data                 | `c.PostForm()`              | **Form → PostForm**    |
| HTTP header               | `c.GetHeader()`             | **Header → GetHeader** |


c.Set()       → c.Get()
middleware   → take it back

/users/:id   → c.Param("id")
URL path     → Param

/users?id=5  → c.Query("id")
?key=value   → Query

JSON body    → c.ShouldBindJSON(&req)

Form body    → c.PostForm("email")

Header       → c.GetHeader("Authorization")
First ask: "Where did this value come from?"

Then choose the method:

Context? → Get
Path?    → Param
Query?   → Query
JSON?    → ShouldBindJSON
Form?    → PostForm
Header?  → GetHeader

# now we are adding ledger system(always double-entry accounting.)
why to know debit and credit of the money 
we cannot  have queries like update and delete to prevent deletion of enteries
You don't want someone doing:

DEBIT User A ₹500

without a corresponding credit.

That would violate double-entry accounting.

Your ledger service should enforce:

SUM(DEBITS) == SUM(CREDITS)

Your query probably looks like:

query := `
    INSERT INTO ledger_entries
    (id, wallet_id, transaction_id, amount, entry_type, balance, created_at)
    VALUES (?, ?, ?, ?, ?, ?, ?)
`

Then:

_, err := tx.ExecContext(
    ctx,
    query,
    ledgerEntry.ID,
    ledgerEntry.WalletID,
    ledgerEntry.TransactionID,
    ledgerEntry.Amount,
    ledgerEntry.EntryType,
    ledgerEntry.Balance,
    ledgerEntry.CreatedAt,
)
Think of it like this

Your struct contains:

ledgerEntry
├── ID
├── WalletID
├── TransactionID
├── Amount
├── EntryType
├── Balance
└── CreatedAt

Your SQL has placeholders:

VALUES (?, ?, ?, ?, ?, ?, ?)

The values after query fill those ? in order:

? → ledgerEntry.ID
? → ledgerEntry.WalletID
? → ledgerEntry.TransactionID
? → ledgerEntry.Amount
? → ledgerEntry.EntryType
? → ledgerEntry.Balance
? → ledgerEntry.CreatedAt

So you're essentially saying:

"Take these values from my ledgerEntry object and put them into the corresponding placeholders in the SQL query."

Why not pass the whole struct?

Because database/sql doesn't automatically take an arbitrary Go struct and map its fields to SQL placeholders.

This:

tx.ExecContext(ctx, query, ledgerEntry)

doesn't mean:

"Insert all fields of this struct."

You explicitly provide the values:

ledgerEntry.ID
ledgerEntry.WalletID
ledgerEntry.TransactionID

and functions should be 
type LedgerService interface {

    // Create and post a balanced financial transaction.
    CreateTransaction(...)

    // Get a transaction and its entries.
    GetTransaction(...)

    // Get ledger history for an account.
    GetAccountEntries(...)

    // Get current balance.
    GetBalance(...)

    // Reverse a previously posted transaction.
    ReverseTransaction(...)
}
# One important thing for insert in mysql using a model struct

The order matters.

If your SQL is:

(id, wallet_id, transaction_id, amount, entry_type, balance, created_at)

your Go arguments must correspond to exactly that order:

ledgerEntry.ID,
ledgerEntry.WalletID,
ledgerEntry.TransactionID,
ledgerEntry.Amount,
ledgerEntry.EntryType,
ledgerEntry.Balance,
ledgerEntry.CreatedAt,

So the simple mental model is:

SQL ? placeholders ← values from your struct fields.

And ExecContext then sends the query + those values to the database inside your transaction.

# sql query of one row and many rows
INSERT / UPDATE / DELETE
        ↓
ExecContext()

SELECT one row
        ↓
QueryRowContext()

SELECT many rows
        ↓
QueryContext()

# optimistic and pessimistic locking in db
--race condition and double payment(idenmptoncy)
Two requests arrive at almost the same time:

Request A → withdraw ₹700
Request B → withdraw ₹500

Both requests might read:

₹1000

If they both think the money is available, you can end up allowing:

₹700 + ₹500 = ₹1200

even though the user only has ₹1000.

That's a race condition.
Optimistic and pessimistic locking are two ways to prevent this.
1. so the idea of optimistic locking is " optimistic lock assumes that the nobody changes the row while i was working ..i will check whether it's still the same version i was working before update  and no thread blocking so faster" so we keep a version number for the wallet here in this project
2.pessimistic lock : locking row while processing,less concurrent means prevent other dependent operation to wait,lock the database,untill transaction done,safer but slow due to blocking and queueing also


# Sentinel Errors vs NewAppError
Sentinel error

A sentinel error is a predefined error value representing a specific, recognizable condition.

var ErrWalletNotFound = errors.New("wallet not found")

Instead of creating a new error every time:

errors.New("wallet not found")

we reuse the same known error:

return ErrWalletNotFound

This allows upper layers to recognize it:

if errors.Is(err, ErrWalletNotFound) {
    // handle wallet-not-found case
}
NewAppError

NewAppError creates an application/API-level error containing information that should eventually be exposed to the client:

NewAppError(
    http.StatusNotFound,
    "WALLET_NOT_FOUND",
    "Wallet not found.",
)

It contains:

StatusCode → HTTP response status
Code       → API error code
Message    → client-facing message
Clean separation

Ideally:

Repository
    ↓
ErrWalletNotFound
ErrConcurrentUpdate
    ↓
Service
    ↓
NewAppError(404, ...)
NewAppError(409, ...)
    ↓
Handler / Error Middleware
    ↓
HTTP Response

Repository says WHAT happened.

ErrWalletNotFound

Service/application layer decides HOW the API should expose it.

404 + WALLET_NOT_FOUND

Important rule

Don't create NewAppError repeatedly for the same known condition. Define a reusable sentinel when the condition needs to be recognized repeatedly.

# idempotency id
An idempotency key is a unique key sent by the client to make sure that retrying the same request does not perform the operation twice.

For a wallet/payment system, this is extremely important.

The problem

Suppose the user wants to transfer ₹500:

POST /api/v1/transfers


₹500 from A → B

Server processes it:

A: ₹1000 → ₹500
B: ₹500  → ₹1000

But then the network fails before the response reaches the client.

The client doesn't know whether the transfer succeeded.

So it retries:

POST /api/v1/transfers


₹500 from A → B

Without protection:

First request:
A ₹1000 → ₹500


Second request:
A ₹500 → ₹0

💥 User got charged twice.

Idempotency key solves this

Client generates a unique key:

Idempotency-Key: 7f3a8c21-91b2-4c5e-a123-abc123

Request:

POST /api/v1/transfers
Idempotency-Key: 7f3a8c21-91b2-4c5e-a123-abc123

Server processes it:

key = 7f3a8c21...
       ↓
not seen before
       ↓
perform transfer
       ↓
store key + result

Then the network fails.

Client retries with the same key:

POST /api/v1/transfers
Idempotency-Key: 7f3a8c21-91b2-4c5e-a123-abc123

Server checks:

Have I already processed this key?
        ↓
       YES
        ↓
Don't perform transfer again
        ↓
Return the previous result

So:

First request
    ↓
₹500 transferred
    ↓
response lost


Retry
    ↓
same idempotency key
    ↓
already processed
    ↓
NO second transfer
How it fits your SudoWallet

Your transactions table could have:

CREATE TABLE transactions (
    id VARCHAR(36) PRIMARY KEY,
    idempotency_key VARCHAR(100) NOT NULL,
    ...
);

And ideally:

UNIQUE (idempotency_key)

So:

Idempotency Key
       ↓
Unique transaction
       ↓
Wallet updates
       ↓
Ledger entries

The database uniqueness constraint becomes your final safety net.

Example

First request:

Idempotency-Key: ABC123

creates:

Transaction ID: TX001
Idempotency Key: ABC123
Status: COMPLETED

Retry:

Idempotency-Key: ABC123

Database says:

ABC123 already exists

So you don't create another transaction.

Idempotency ≠ concurrency control

They're solving different problems.

Optimistic locking

Protects against:

Two requests modifying the same wallet concurrently.

version 5
   ↓
A succeeds → version 6
B has version 5 → conflict
Idempotency key

Protects against:

The same logical request being submitted multiple times.

Request ABC123
    ↓
transfer ₹500


Retry ABC123
    ↓
don't transfer again

In a real wallet system, you want both:

                    Transfer Request
                           │
                 Idempotency Key
                           ↓
                 "Have I processed this?"
                           │
                    ┌──────┴──────┐
                   NO             YES
                    ↓              ↓
                 process       return previous result
                    │
                    ↓
              DB transaction
                    │
             ┌──────┴──────┐
             ↓             ↓
        concurrency      ledger
          control        entries

# layer was error handling 
GO ERROR HANDLING IN CLEAN ARCHITECTURE

Repository
──────────
• Talks to DB.
• Unexpected DB failure → return original err.
• Known recognizable condition → return sentinel/domain error.
• Avoid HTTP-specific errors here.

Examples:
    return err
    return ErrWalletNotFound
    return ErrConcurrentUpdate


Service
───────
• Contains business logic.
• Recognizes known errors using errors.Is().
• Converts them into application/API errors when appropriate.

Example:
    if errors.Is(err, ErrWalletNotFound) {
        return NewAppError(404, "WALLET_NOT_FOUND", "Wallet not found.")
    }


Handler
───────
• Calls service.
• Usually forwards errors to middleware.

    c.Error(err)


Middleware
──────────
• Converts AppError into HTTP response.
• Uses errors.As() to identify AppError.

    AppError
       ↓
    StatusCode
    Code
    Message
       ↓
    HTTP response

# why sender type need to be *string in transaction model
a transaction can legitimatly cannot have sender in case of topup of wallet
but issue when database/sql  reading NULL values.
in case of createTx if senderwalletid ==nil so db understand nil as sql null

Go                         MySQL
SenderWalletID == nil  →  NULL
SenderWalletID != nil  →  actual wallet ID

For example, a deposit:
sender_wallet_id   = NULL
receiver_wallet_id = wallet-123
works.

-- GetByIdempotencyKey — this is where you need to be careful
Suppose MySQL returns:sender_wallet_id = NULL
If you do:.Scan(&transaction.SenderWalletID)
you can run into a conversion problem because SQL NULL isn't directly a Go string.The clean solution is to use sql.NullString while scanning.var senderWalletID sql.NullString

// sender_wallet_id can be NULL for transactions such as top-ups.
// A SQL NULL value cannot be scanned directly into a Go string,
// because a string cannot represent SQL NULL.
//
// Therefore, we use sql.NullString as the scan destination.
// sql.NullString contains:
//   - String: the actual string value
//   - Valid: whether the database value is NOT NULL
//
// If the database value is NULL:
//   Valid = false
//
// If the database value contains a string:
//   Valid = true
//   String = the actual wallet ID
# valid check of sender validity

SQL NULL
   ↓
Valid = false
   ↓
don't assign
   ↓
*string remains nil

while:

SQL string
   ↓
Valid = true
   ↓
assign pointer
   ↓
*string contains wallet ID

That's the whole purpose of the if sender.Valid check.

# some json tags
required → "must exist / not be empty"

gt=0 → "must be greater than 0"

gte=0 → "must be greater than or equal to 0"

lt=100 → "must be less than 100"

lte=100 → "must be less than or equal to 100"



*string contains wallet ID

That's the whole purpose of the if sender.Valid check.

# day 10: agenda
to implement soft delete(in order to save referential integrity) like a user can have important data in other table so if we delete then we create referential integrity problem.
so basically delete from users;-> should mark is_deleted(true/false)

pagination and sorting as there will be millions of enteries so slow and take time a query so it affects api response time/

file upload for avatar in wallet profile picture

## json tag based on situation
JSON request body  → json:"..."
Query parameters   → form:"..."
Path parameters    → uri:"..."

so dto for pagination becomes 
// Input from URL query
type PaginationParams struct {
    Page   int    `form:"page,default=1"`          // which page client wants
    Limit  int    `form:"limit,default=10"`        // how many records per page
    Sort   string `form:"sort,default=created_at"`  // which column to sort by
    Order  string `form:"order,default=desc"`       // order: asc/desc
    Status string `form:"status"`                   // optional filter
}
//output as json
type PaginationMeta struct {
    Page      int `json:"page"`       // current page
    TotalPage int `json:"total_page"` // total number of pages
    Limit     int `json:"limit"`      // records per page
    TotalData int `json:"total_data"` // total number of records
}

## genric type for paginated response instead of any 
With generics, the type of Data is known at compile time.

For transaction history:

PaginatedResponse[model.Transaction]

means:

Data []model.Transaction

For users:

PaginatedResponse[model.User]

means:

Data []model.User

So you get type safety.

any version
type PaginatedResponse struct {
    Success bool           `json:"success"`
    Data    any            `json:"data"`
    Meta    PaginationMeta `json:"meta"`
}

This is more flexible:

PaginatedResponse{
    Data: transactions,
}

But Data is essentially:

interface{}

The compiler doesn't know what is supposed to be inside it.

In your wallet project

You could have:

transactions := []model.Transaction{...}

response := dto.PaginatedResponse[model.Transaction]{
    Success: true,
    Data:    transactions,
    Meta:    meta,
}

And for users:

users := []model.User{...}

response := dto.PaginatedResponse[model.User]{
    Success: true,
    Data:    users,
    Meta:    meta,
}

Same response structure, different data type.

Simple rule
any
↓
"I don't care what type this is."

[T any]
↓
"I want this response to work with different types,
but I still want Go to know the exact type."

So for a new Go project, I'd choose:

type PaginatedResponse[T any] struct {
    Success bool           `json:"success"`
    Data    []T            `json:"data"`
    Meta    PaginationMeta `json:"meta"`
}

It's cleaner and gives you compile-time type safety.
type PaginatedResponse[T any] struct {
    Success bool           `json:"success"`
    Data    []T            `json:"data"`
    Meta    PaginationMeta `json:"meta"`
}

any version
type PaginatedResponse struct {
    Success bool           `json:"success"`
    Data    any            `json:"data"`
    Meta    PaginationMeta `json:"meta"`
}

This is more flexible:

PaginatedResponse{
    Data: transactions,
}

But Data is essentially:

interface{}

The compiler doesn't know what is supposed to be inside it.

Note:
any
↓
"I don't care what type this is."

[T any]
↓
"I want this response to work with different types,
but I still want Go to know the exact type."

-- used this dto as common since we can have need of this in transactions,users so made it common

# how to design getHistory function
suppose a wallet have 47 txn then we cannot return all the transactions at one this slows the query since querying over rows
With pagination:
page = 1
limit = 10

you want:
transactions 1–10
Page 2:
transactions 11–20
Page 3:
transactions 21–30 and so on

so SQL implements this with:
LIMIT 10 OFFSET 10

because:
offset = (page - 1) × limit
Therefore:
page 1 → (1 - 1) × 10 = 0
page 2 → (2 - 1) × 10 = 10
page 3 → (3 - 1) × 10 = 20

so with paginationparam dto and offset function
a  request becomes GET /api/v1/transactions?page=2&limit=10
Page   = 2
Limit  = 10
Offset = 10

The repository needs enough information to answer:
"Give me this wallet's transactions, with these pagination/filter/sort parameters."
so GetHistory(
    ctx context.Context,
    walletID string,
    params dto.PaginationParams,
) ([]*model.Transaction, int64, error)

[]*model.Transaction → current page's transactions
int64                → total matching transactions
error                → database error

now query becomes SELECT ...
FROM transactions
WHERE sender_wallet_id = ?
   OR receiver_wallet_id = ?
ORDER BY created_at DESC, id DESC
LIMIT ? OFFSET ?;


and getHostory function will be doing
This GetHistory() is doing four main jobs:

1. Count total matching transactions
2. Build the paginated SELECT query
3. Read the current page from sql.Rows
4. Return current-page transactions + total count


# how to use migration tool to create migartion up and down file
migrate create -ext <extension> -dir <directory> -seq <migration_name>

migrate create
      │
      ├── -ext sql       → file type
      ├── -dir ...      → where
      ├── -seq           → numbering
      └── name           → what this migration is about

the resulting pair is:
000001_create_users_table.up.sql    ← apply
000001_create_users_table.down.sql  ← rollback


# how to get the connection string of database(docker image)

# Host machine
mysql://sudowallet_user:sudowallet_password@tcp(localhost:3306)/sudowallet

# Another Docker container(look for yaml, cat docker-compose.yml )
mysql://sudowallet_user:sudowallet_password@tcp(mysql:3306)/sudowallet

genreal formula: mysql://<MYSQL_USER>:<MYSQL_PASSWORD>@tcp(<HOST>:<PORT>)/<MYSQL_DATABASE>

| Database        | Connection string                         |
| --------------- | ----------------------------------------- |
| **PostgreSQL**  | `postgres://USER:PASSWORD@HOST:PORT/DB`   |
| **MySQL**       | `mysql://USER:PASSWORD@tcp(HOST:PORT)/DB` |
| **MongoDB**     | `mongodb://USER:PASSWORD@HOST:PORT/DB`    |
| **Redis**       | `redis://:PASSWORD@HOST:PORT/DB_NUMBER`   |
| **CockroachDB** | `postgresql://USER:PASSWORD@HOST:PORT/DB` |

The connection-string structure changes by database. Here's the cheat sheet.

PostgreSQL
postgres://<USER>:<PASSWORD>@<HOST>:<PORT>/<DATABASE>?sslmode=disable

Example:

postgres://postgres:password@localhost:5432/myapp?sslmode=disable

For migrate:

migrate -path db/migrations \
  -database "postgres://postgres:password@localhost:5432/myapp?sslmode=disable" \
  up
MySQL
mysql://<USER>:<PASSWORD>@tcp(<HOST>:<PORT>)/<DATABASE>

Example:

mysql://root:password@tcp(localhost:3306)/myapp
MongoDB

MongoDB uses a different URI format:

mongodb://<USER>:<PASSWORD>@<HOST>:<PORT>/<DATABASE>

Example:

mongodb://admin:password@localhost:27017/myapp

Without authentication:

mongodb://localhost:27017/myapp

For MongoDB Atlas, you'll commonly see:

mongodb+srv://<USER>:<PASSWORD>@<CLUSTER>/<DATABASE>
Redis

Redis usually doesn't have a database name in the same sense as PostgreSQL/MySQL.

Basic:

redis://<HOST>:<PORT>

Example:

redis://localhost:6379

With password:

redis://:<PASSWORD>@<HOST>:<PORT>

Example:

redis://:mypassword@localhost:6379

With a Redis logical DB:

redis://:<PASSWORD>@localhost:6379/0

Here /0 means Redis database number 0.

CockroachDB

CockroachDB speaks the PostgreSQL wire protocol, so its connection string is basically PostgreSQL-style:

postgresql://<USER>:<PASSWORD>@<HOST>:<PORT>/<DATABASE>?sslmode=require

Example:

postgresql://root@localhost:26257/myapp?sslmode=disable

For a secured CockroachDB cluster:

postgresql://user:password@host:26257/myapp?sslmode=verify-full

NOTE: The migration files are essentially the version history of your database schema.

## migrate go-tool workflow
We did it using golang-migrate. The workflow we followed was:

1. Create a new migration

From your project root:

migrate create -ext sql -dir db/migrations -seq add_avatar_url_to_users

This creates:

db/migrations/
├── 000002_add_avatar_url_to_users.up.sql
└── 000002_add_avatar_url_to_users.down.sql
2. Write the schema change

000002_add_avatar_url_to_users.up.sql:

ALTER TABLE users
ADD COLUMN avatar_url VARCHAR(255) NULL
AFTER password_hash;

000002_add_avatar_url_to_users.down.sql:

ALTER TABLE users
DROP COLUMN avatar_url;
3. Get the database connection string

For your Docker MySQL setup:

mysql://sudowallet_user:sudowallet_password@tcp(localhost:3306)/sudowallet

Because migrate is running on your host, we use localhost.

4. Apply the migration
migrate -path db/migrations \
  -database "mysql://sudowallet_user:sudowallet_password@tcp(localhost:3306)/sudowallet" \
  up

This changes:

users
├── id
├── email
├── password_hash
└── avatar_url       ← new
5. Check the migration version
migrate -path db/migrations \
  -database "mysql://sudowallet_user:sudowallet_password@tcp(localhost:3306)/sudowallet" \
  version

Expected:

2
6. Roll back if needed

Because we created the down.sql:

migrate -path db/migrations \
  -database "mysql://sudowallet_user:sudowallet_password@tcp(localhost:3306)/sudowallet" \
  down 1

That executes:

ALTER TABLE users
DROP COLUMN avatar_url;
The complete mental model
You change your Go application's schema requirement
                ↓
       migrate create
                ↓
       000003_xxx.up.sql
       000003_xxx.down.sql
                ↓
          Write SQL
                ↓
        migrate ... up
                ↓
       Database schema v3
                ↓
       Commit migration files
                ↓
       Other environments
                ↓
        migrate ... up

So whenever you need a new database schema version, remember:

CREATE → WRITE SQL → UP → VERSION → COMMIT

And don't edit an old migration that's already been applied/shared. Create a new migration instead:

migrate create -ext sql -dir db/migrations -seq <new_change>

my migration workflow 

# difference between 
err = s.userRepo.UpdateAvatar(ctx, id, avatarURL)

if err != nil {
    return customErr.ErrInternalServer
}

return nil

You already have an err variable from earlier:

_, err := s.userRepo.GetById(ctx, id)

So you're reusing that variable.

2. Short if version
if err := s.userRepo.UpdateAvatar(ctx, id, avatarURL); err != nil {
    return customErr.ErrInternalServer
}

return nil

Here, err is declared specifically for the if statement.


# how to send image in http request

an image is binary data, so for file uploads we commonly use:

Content-Type: multipart/form-data

With multipart/form-data, the request looks conceptually like:

POST /users/avatar HTTP/1.1
Content-Type: multipart/form-data; boundary=XYZ

Then the body is split into parts:

--XYZ
Content-Disposition: form-data; name="user_id"

123

--XYZ
Content-Disposition: form-data; name="avatar"; filename="avatar.jpg"
Content-Type: image/jpeg

<binary image data>

--XYZ--

So there are two parts:

┌─────────────────────────┐
│ user_id                 │
│ 123                     │
├─────────────────────────┤
│ avatar                  │
│ avatar.jpg              │
│ <binary data>           │
└─────────────────────────┘

-- to get the file url from context we need to do 
file,err:=c.FormFile("avatar)

## How to get the image url for avatar
HTTP Request
     ↓
multipart/form-data
     ↓
Gin
     ↓
c.FormFile("avatar")
     ↓
Validate file
     ↓
Upload to S3 / Cloudinary / storage
     ↓
Get URL
     ↓
UPDATE users
SET avatar_url = ?
     ↓
Database


# how to upload file and save file in local project directory
Hander layer:
func (h *UserHandler) UpdateAvatar(c *gin.Context) {
	UserID := c.GetString("userID")
	//get the file from the request multipart form data
	file, err := c.FormFile("avatar")
	if err != nil {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_FILE", "Please upload an avatar"))
		return
	}
	//save the file to the server
	avatarPath := "./uploads/" + file.Filename
	if err := c.SaveUploadedFile(file, avatarPath); err != nil {
		c.Error(customErr.NewAppError(http.StatusInternalServerError, "Failed to save avatar", err.Error()))
		return
	}
	//validate the file type and size if needed (e.g., only allow images, limit size)
	if file.Size > 5*1024*1024 { // 5MB limit
		c.Error(customErr.NewAppError(http.StatusBadRequest, "FILE_TOO_LARGE", "Avatar file size should be less than 5MB"))
		return
	}
	//validate the file type
	ext := file.Header.Get("Content-Type")
	if ext != "image/jpeg" && ext != "image/png" && ext != "image/gif" {
		c.Error(customErr.NewAppError(http.StatusBadRequest, "INVALID_FILE_TYPE", "Only JPEG, PNG, and GIF files are allowed"))
		return
	}
	//folder for storing avatars
	uploadDir := "./uploads"
	// ModePerm sets the file mode to 0777, which means that the directory is readable, writable, and executable by everyone. This is generally not recommended for production environments due to security concerns. You might want to set more restrictive permissions based on your application's needs or u can explicitly set the permissions to 0755 or 0700 based on your requirements. For example, you can use os.ModeDir | 0755 to create a directory that is readable and executable by everyone but writable only by the owner.
	_ = os.MkdirAll(uploadDir, os.ModePerm)
	//reanme the file based on the user id
	filename := UserID + ext
	//destination
	destination := filepath.Join(uploadDir, filename)
	//save the file
	if err := c.SaveUploadedFile(file, destination); err != nil {
		c.Error(customErr.ErrInternalServer)
		return
	}
	//update user avatar
	avatarURL := "/uploads/" + filename
	if err := h.svc.UpdateAvatar(c.Request.Context(), UserID, avatarURL); err != nil {
		c.Error(customErr.ErrInternalServer)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Avatar updated successfully","avatar_url": avatarURL})
}

Note:
	//use can use 0777 or 07775 or os.ModePerm 
	//difference between mkdirAll and mkdir is that mkdirall will create all the parent directories if they do not exist, whereas mkdir will return an error if the parent directory does not exist. So we use mkdirall here to create the uploads directory if it does not exist.


# Unit testing

Unit testing is the practice of testing the smallest testable parts of your code (called units, usually functions or methods) in isolation to ensure they work correctly.

Why unit testing?
Detect bugs early
Make code changes safer
Improve code quality
Serve as documentation for how code should behave
## Good Unit Test Characteristics

A good unit test should be:

Independent: Doesn't rely on other tests.
Fast: Runs in milliseconds.
Repeatable: Produces the same result every time.
Readable: Easy to understand.
Focused: Tests one behavior at a time.
## Basic Process
Write a small function.
Create a test for that function.
Run the test.
Fix the code if the test fails.
Repeat.

## what to test 
Normal inputs
Boundary values
Invalid inputs
Edge cases
Expected exceptions
## patterns to follow 
AAA(arrange,Act,Assert)

def test_add():
    # Arrange
    a = 2
    b = 3

    # Act
    result = add(a, b)

    # Assert
    assert result == 5

## testify and testify/mock golang package to test 
go get github.com/stretchr/testify

Testify is a testing toolkit built on top of Go's testing package that gives you:

easier assertions → assert
fatal assertions → require
mocks/expectations → mock
test suites → suite

testify gives three tools
1.Assert
2.Require
3.Mock

assert
"Is this result correct?"
assert.Equal(t, 5, result)

require
"If this isn't correct, stop immediately."
require.NoError(t, err)


mock
"Pretend this dependency exists and control what it returns."
mockRepo.
	On("GetUser", "123").
	Return(user, nil)


# swagger
how to write annotation for handler endpoints
What is this endpoint?
        ↓
@Summary
@Description

Who does it belong to?
        ↓
@Tags

What comes in?
        ↓
@Param

What goes out?
        ↓
@Success
@Failure

Where is it?
        ↓
@Router


# swagger docs building command 
`swag init -g monolith/cmd/main.go -o monolith/docs --parseInternal`


# so our plan is to implement redis for 
1.get wallet/me from redis if not then db query
2.always initiate on transfer balance increase or decrease we must delete the cache key(cache invalidation) in redis so next balance returns from db
You invalidate when the underlying wallet data changes.
For a transfer:

Sender:   ₹1000 → ₹700
Receiver: ₹500  → ₹800

You delete both cache keys:

cache.Delete(ctx, senderID)
cache.Delete(ctx, receiverID)
## NOTE:redis always injected in service layer and passed to the requested services

-- take a image of redis in docker 
--configure the redis port and url in env and config so it can be loaded in config struct
--then create the redis wrapper in database
--then initalize in main.go with taking configs from config layer
--apply redis on service layers
Redis is an optimization; PostgreSQL is the source of truth.

# cache concepts
DB updated successfully, but Redis cache is stale or invalidation failed.



1. Retrying cache invalidation

Your current flow:

DB COMMIT ✅
     ↓
Redis DEL ❌
     ↓
Cache remains stale

Retry means:

DB COMMIT ✅
     ↓
Redis DEL ❌
     ↓
Retry
     ↓
Redis DEL ✅

Example:

err := cache.Delete(ctx, senderID, receiverID)

if err != nil {
    // retry later
}

Could retry:

1st attempt → immediately
2nd attempt → 100ms
3rd attempt → 500ms
4th attempt → 1s

Usually with exponential backoff.

Problem solved

Temporary Redis failure.

2. TTL as a safety net

You already have:

redis.Set(ctx, key, data, 5*time.Minute)

That 5*time.Minute is the TTL.

Suppose:

DB = ₹700
Redis = ₹1000

and DEL fails.

Eventually:

5 minutes pass
      ↓
Redis key expires
      ↓
GET wallet
      ↓
Redis MISS
      ↓
DB = ₹700
      ↓
Redis SET ₹700

So TTL acts as a maximum lifetime for stale data.

Important

TTL does not guarantee consistency.

For 5 minutes, you could still serve stale data.

It's just a safety net.

3. Outbox Pattern

This is a much bigger concept.

Problem:

DB transfer ✅
Redis DEL ❌

How do you make sure the invalidation isn't simply lost?

You write an event into the database in the same transaction:

DB Transaction
│
├── Update sender balance
├── Update receiver balance
└── Insert event:
       "WalletChanged(sender, receiver)"
             ↓
          COMMIT

Now you have:

PostgreSQL
├── wallet changes
└── outbox event

A background worker continuously reads the outbox:

Outbox Worker
      ↓
Read event
      ↓
Redis DEL
      ↓
Success → mark event processed

If Redis is down:

Redis ❌
   ↓
event remains in DB
   ↓
worker retries later
   ↓
Redis ✅
   ↓
DEL

This is extremely useful because the invalidation instruction isn't lost.

4. Asynchronous invalidation

Currently you're doing:

Transfer request
      ↓
DB COMMIT
      ↓
Redis DEL
      ↓
Response

That's synchronous.

Asynchronous means:

Transfer
   ↓
DB COMMIT
   ↓
Queue/event
   ↓
Response to user

Then:

Background worker
      ↓
Redis DEL

For example:

Transfer Service
      ↓
Kafka / RabbitMQ / SQS
      ↓
Cache Invalidation Worker
      ↓
Redis DEL
Why?

Your user doesn't have to wait for Redis invalidation.

But there's a tradeoff:

The cache might remain stale for a short period.

5. Versioned cache entries

This one is interesting.

Instead of simply:

wallet:user:123 → ₹1000

you associate a version:

DB:
balance = ₹700
version = 8

Cache:

wallet:user:123
balance = ₹1000
version = 7

Now your application can determine:

DB version = 8
Cache version = 7

Therefore:

Cache is stale

You can use versions/timestamps/sequence numbers to prevent older cached data from overwriting newer data.

This becomes particularly useful in distributed systems where multiple processes can update the same data.

6. Cache stampede protection

This is a completely different problem.

Imagine your wallet cache expires:

wallet:user:123
      ↓
TTL expires
      ↓
CACHE MISS

But suppose 10,000 requests arrive at exactly that moment.

Without protection:

10,000 requests
       │
       ├── DB query
       ├── DB query
       ├── DB query
       ├── DB query
       ├── DB query
       └── ...

Your DB gets hammered.

That's a cache stampede.

Protection

Make only one request fetch from DB:

10,000 requests
       │
       ▼
     Redis
       │
      MISS
       │
       ▼
   Lock / singleflight
       │
       ▼
  Request #1 → DB
       │
       ▼
   Redis SET
       │
       ▼
Other 9,999 requests → Redis

In Go, singleflight is a nice tool for protecting against this within a single application instance.


# go func() starts an anonymous function as a goroutine, allowing it to execute concurrently without blocking the current goroutine.
so we are using go routine for cache invalidation instead of message broker->cache worker-?if cache worker fails  it restarts and then try invalidation again
here with go routine we are not blocking the transfer request saying doesnot wait for cache to delete (Del() to finish.)
and this make this del() a non-blocking from the caller's perspective.
never say go func() launches a thread
It launches a goroutine. Goroutines are lightweight units of concurrent execution managed by the Go runtime, rather than directly mapping each goroutine to an OS thread.

## With a message broker

Instead:

DB COMMIT
    ↓
Publish event
    ↓
RabbitMQ / Kafka / SQS
    ↓
HTTP response

Then:

Broker
   ↓
Cache Worker
   ↓
Redis DEL

If the worker crashes:

Broker
   ↓
message remains/unacked
   ↓
worker restarts
   ↓
retry

That's the major difference.


# race condition while cache invalidation
//Now a separate goroutine is writing to it.
// That can create a data race if the outer function also accesses err.
//2 redis calls but can be done in one call
		err = s.redisClient.Del(context.Background(), senderKey,receiverKey).Err()
		err = s.redisClient.Del(context.Background(), receiverKey).Err()

//make it one 
        go func() {
    bgCtx := context.Background() //give this background job their own lifecycle managed context

    err := s.redisClient.Del(
        bgCtx,
        senderKey,
        receiverKey,
    ).Err()

    if err != nil {
        // log
    }
}()
# So using the request context for fire-and-forget work is usually not what you want. cause as request get served and still background job is not finished then context  get cancelled and this may kill background job too 
we want to give background job an independent own lifecycle-managed context so use ctx.Background() but dont use this also blindly.
And when the application shuts down:

SIGTERM
  ↓
worker context cancelled
  ↓
background jobs stop gracefully

# new agenda 
1.Rate limiting 
max request in a min if exceed retuen an error
save the rate limit key in redis with ip address of the user and rate limit for time (ttl)
2.jwt blacklisting 
--create logout api to blacklist jwt in redis
--we create a blacklist and save the token there 
if token is blacklisted in cache then return unauthorized


# single opertion vs batch operation in redis
Single operation:

count, err := redisClient.Incr(ctx, key).Result()

✅ Best choice
✅ Simple
✅ Atomic increment
✅ One Redis round trip

A pipeline: is useful when you have multiple independent Redis commands that you want to send together:

pipe := redisClient.Pipeline()

incrCmd := pipe.Incr(ctx, key)
pipe.Expire(ctx, key, time.Minute)

_, err := pipe.Exec(ctx)
if err != nil {
    // handle error
}

count := incrCmd.Val()

Here pipeline makes sense because you're doing:

INCR
  +
EXPIRE

in one batch.

But there's an important Redis detail

If your goal is a rate limiter/counter with TTL, don't blindly do:

INCR
EXPIRE

as two independent operations without thinking about atomicity.

For example:

Request 1 → INCR
Request 2 → INCR
Request 1 → EXPIRE
Request 2 → EXPIRE

The TTL can be refreshed unexpectedly depending on your design.

# errros throwing
if your error middleware is responsible for converting errors into HTTP responses.
c.Error(customErr.NewAppError(...))
c.Abort()

2nd 
Agar tumhari global error middleware c.Errors ko process karti hai,
c.Error(customErr.ErrInternalServer)
c.Abort()


# sinle operation vs pipeline vs lua in redis. which to use to avoid netwoek round trips?
| Approach             | Kya karta hai?                                                |                  Atomic?|Bestuse                                        |

| **Single operation** | Ek Redis command                                              | Usually ✅, command-level | Simple `GET`, `SET`, `INCR`, `DEL`               |
| **Pipeline**         | Multiple commands ko batch karta hai                          |                        ❌ | Multiple independent commands, fewer round trips |
| **Lua**              | Multiple commands + logic ko Redis ke andar execute karta hai |                        ✅ | Conditional/multi-step atomic logic              |


The easiest mental model
Single operation

"Mujhe ek kaam karna hai."

redis.Incr(...)
Pipeline

"Mujhe multiple kaam karne hain, aur network round trips kam karne hain."

INCR
EXPIRE
GET
Lua

"Mujhe multiple kaam + conditions/logic chahiye, aur sab ek atomic operation hona chahiye."

INCR
↓
IF
↓
EXPIRE
For your rate limiter

Your current:

INCR + EXPIRE

Pipeline → batching, but not atomic.

More robust:

Lua:
INCR
IF first request
    EXPIRE
RETURN count


```
Single command → single operation.
Multiple independent commands → pipeline.
Multiple dependent/conditional commands that must be atomic → Lua.
```  
# window in time
 here window mean Window = "kitne time ke block mein requests count karni hain"
Suppose requirement hai:User maximum 10 requests per 1 minute kar sakta hai.
Yahan:limit  = 10 requests window = 1 minute
Matlab hum requests ko 1-minute ke buckets mein count karenge.
agar usne ek min ke nadar 10th req tak allow karenge then 11th blockn
lekin jaise he 1 min ke baad new window aaya toh hum uske liye fir se 10 requests allow karenge

aur ye currentwindow kaise calculate karenge? Suppose window = 1 minute, then currentWindow = time.Now().Unix() / int64(window.Seconds())
ye hame humein current window ka ID deta hai.( unix time divided by window duration in seconds) Suppose current time is 12:00:30, then currentWindow = 12:00:30 / 60 = 20.5 => 20
so key becomes rate_limit:192.168.1.10:29384720 then next min key becomes rate_limit:192.168.1.10:29384721
and new key means new counter

 ab ttl and window ka connection kya hai?
 window =1min and ttl=60sec
first req arrives key is created with ttl=60sec, then next req comes within 60 sec then count is incremented, but after 60 sec key is expired and new key is created for new window

### Rate Limiter ka precise logic

Humne agar **window = 1 minute** aur **limit = 10 requests** define kiya hai, iska matlab hai ki user ek fixed 1-minute window ke andar maximum **10 requests** kar sakta hai. 11th request par user ko `429 Too Many Requests` milega aur us window ke khatam hone ka wait karna padega. Jaise hi next 1-minute window start hogi, ek **new window** create hogi aur user ko phir se 10 requests ki permission milegi.

**TTL ka kaam request count karna ya ye check karna nahi hai ki user ne next request ki ya nahi.** TTL sirf Redis key ki lifetime decide karta hai.

Pehli request aane par:

```text
INCR → counter = 1
EXPIRE → key ka TTL = 60 seconds
```

Agar user subsequent requests karta hai, to sirf counter increment hota hai:

```text
Request 1 → counter = 1
Request 2 → counter = 2
Request 3 → counter = 3
...
Request 10 → counter = 10
Request 11 → counter = 11 → reject
```

TTL har request par reset nahi hota, kyunki Lua script mein `EXPIRE` sirf tab execute hota hai jab:

```text
count == 1
```

Agar user pehli request ke baad koi aur request nahi karta, tab bhi 60 seconds complete hone ke baad Redis automatically us rate-limit key ko delete kar dega.

Isliye:

```text
Window → requests kis time bucket mein count hongi
Limit  → us window mein maximum kitni requests allowed hain
INCR   → current window mein kitni requests aayi hain
TTL    → Redis key ko kab automatically delete karna hai
Lua    → INCR + first-request par EXPIRE ko atomically execute karta hai
```

# now blacklisting of jwt token in redis after logout in auth middleware 
--apply before validating the token if it's in redis cache blacklist




# cron job trick
 Each star stands for a different unit of time.text
  *   *   *   *   *
 │   │   │   │   │
 │   │   │   │   └─── Day of the Week (0 - 7) (Sunday is 0 or 7)
 │   │   │   └─────── Month of the Year (1 - 12)
 │   │   └─────────── Day of the Month (1 - 31)
 │   └─────────────── Hour (0 - 23)
 └─────────────────── Minute (0 - 59)

* (The Any Symbol): Means "every single time." A star in the minute place means "every minute."
, (The And Symbol): Links specific times together. 1,15 in the hour place means "at 1 AM and 3 PM."
/ (The Every X Symbol): Sets a repeating gap. */5 in the minute place means "every 5 minutes."

e.g
0 0 * * * 
Minute 0, Hour 0. This runs exactly at midnight every single day.

*/15 8-17 * * *
Every 15 minutes, between the hours of 8 AM and 5 PM, every day.

0 12 * * 1

Minute 0, Hour 12, Weekday 1 (Monday). This runs every Monday at noon

Use Word ShortcutsIf you hate numbers, many systems let you replace all 5 stars with a single word! You can just write these instead:
@hourly (Runs once an hour)
@daily (Runs once a day at midnight)
@weekly (Runs once a week on Sunday midnight)
@monthly (Runs once a month on the 1st)




# Go Context — Quick Notes

## 1. What is `context`?

`context.Context` Go mein request/operation ke **lifecycle, cancellation, deadline aur timeout** ko control karne ke liye use hota hai.

Commonly pass kiya jata hai:

```go
func DoSomething(ctx context.Context) error
```

---

## 2. `context.Background()`

```go
ctx := context.Background()
```

Ye ek **fresh/root/standalone context** hai.

Isme initially:

* No timeout
* No deadline
* No cancellation

### Use when:

Kisi operation ka parent context nahi hai.

```go
ctx := context.Background()
redisClient.Get(ctx, key)
```

Mental model:

```text
Background()
     ↓
 Fresh / Root Context
```

---

## 3. `r.Context()`

HTTP handler mein:

```go
func Handler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
}
```

`r.Context()` ka context **HTTP request ke lifecycle se attached** hota hai.

Agar client request cancel/disconnect kar deta hai, context cancel ho sakta hai.

```text
Client
   ↓
HTTP Request
   ↓
r.Context()
   ↓
Service
   ↓
Redis / DB
```

### Important:

`r.Context()` standalone nahi hai.

It represents:

> "Ye context current HTTP request ka hai."

---

## 4. `context.WithTimeout()`

```go
ctx, cancel := context.WithTimeout(
    context.Background(),
    10*time.Second,
)

defer cancel()
```

Meaning:

> Is context ke through hone wala operation maximum 10 seconds allowed hai.

10 seconds ke baad:

```go
ctx.Err()
```

returns:

```text
context deadline exceeded
```

Mental model:

```text
Background()
     ↓
WithTimeout(10 sec)
     ↓
New Context
     ↓
Maximum 10 sec
```

---

## 5. Request + Timeout Together

Real backend code mein commonly:

```go
ctx, cancel := context.WithTimeout(
    r.Context(),
    10*time.Second,
)
defer cancel()
```

Meaning:

> Request ke context ko parent rakho, but operation ko maximum 10 seconds ki deadline bhi do.

```text
HTTP Request
     │
     ↓
r.Context()
     │
     ↓
WithTimeout(10 sec)
     │
     ↓
   Service
     │
     ├── Redis
     └── Database
```

Ab operation stop ho sakta hai if:

1. Client request cancel kare
2. 10 seconds complete ho jaye
3. `cancel()` explicitly call ho

---

# 6. `defer cancel()`

```go
ctx, cancel := context.WithTimeout(...)
defer cancel()
```

`cancel()` ko defer karna good practice hai.

Ye context ke associated resources/timer ko release karne mein help karta hai when the operation finishes early.

---

# 7. Quick Comparison

| Context                                    | Meaning                                |
| ------------------------------------------ | -------------------------------------- |
| `context.Background()`                     | Fresh/root standalone context          |
| `r.Context()`                              | Current HTTP request ka context        |
| `context.WithTimeout(ctx, 10*time.Second)` | Existing context + timeout             |
| `context.WithCancel(ctx)`                  | Existing context + manual cancellation |
| `context.WithDeadline(ctx, t)`             | Existing context + fixed deadline      |

---

# 8. Golden Mental Model

```text
context.Background()
        ↓
    Root Context
        ↓
   ┌────┴────┐
   ↓         ↓
WithTimeout  WithCancel
   ↓         ↓
   ctx       ctx
```

Aur HTTP server mein:

```text
HTTP Request
      ↓
  r.Context()
      ↓
WithTimeout(...)
      ↓
Service
      ↓
 Redis / DB / API
```

### One-liner to remember

```text
Background = standalone/root
r.Context() = request lifecycle
WithTimeout = deadline add karo
cancel() = manually stop karo
```

### Typical Go Backend Pattern

```go
func Handler(w http.ResponseWriter, r *http.Request) {

    ctx, cancel := context.WithTimeout(
        r.Context(),
        5*time.Second,
    )
    defer cancel()

    result, err := service.DoSomething(ctx)

    if err != nil {
        // handle error
        return
    }

    // response
}
```

**Interview answer:**

> "Context in Go is used to propagate cancellation, deadlines, timeouts, and request-scoped values across API boundaries and goroutines."


# Graceful shutdown

Graceful shutdown mein hum application ke main goroutine ko directly server ke ListenAndServe() se block nahi karte.

Hum roughly 2 important execution flows rakhte hain:

Server flow — ek goroutine mein ListenAndServe() chalta hai. Ye HTTP server ko run karta hai, incoming connections/requests accept karta hai aur unhe handlers tak pahunchata hai.
Main flow — main goroutine shutdown signal (SIGINT / SIGTERM) ka wait karta hai.

Jab application ko SIGTERM milta hai, main goroutine ko pata chalta hai ki server ko shutdown karna hai.

Ab hum:

server.Shutdown(ctx)

call karte hain.

Shutdown() ka purpose hai:

Naye requests accept karna band karo, lekin jo requests already processing mein hain unhe complete hone ka chance do.

Saath mein hum ek timeout wala context dete hain:

ctx, cancel := context.WithTimeout(
    context.Background(),
    10*time.Second,
)
defer cancel()

Iska matlab:

"Existing work ko maximum 10 seconds do. Agar is duration ke andar graceful shutdown complete ho gaya, great. Agar nahi hua, to Shutdown() aur wait nahi karega."

Yaani context ek tarah ka deadline provide karta hai.

Ab ek important correction

Ye mat sochna ki:

"SIGTERM aaya → hum manually har goroutine ko bolte hain ki shutdown ho jao."

Actually HTTP server ke case mein hum directly har request goroutine ko notify nahi karte.

Hum server ko bolte hain:

server.Shutdown(ctx)

Aur http.Server graceful shutdown process handle karta hai:

Stop accepting new connections
            ↓
Existing requests ko finish hone do
            ↓
Connections close karo
            ↓
Shutdown complete

Agar tumhare application mein background workers / scheduler / Kafka consumer etc. hain, unke liye usually alag shutdown mechanism hota hai—jaise context.Context, Stop(), WaitGroup, etc.

Exact story flow

Ab isko ek real-life story ki tarah visualize karo.

Suppose tumhara SudoWallet server chal raha hai:

                    SudoWallet
                        │
                ┌───────┴────────┐
                │                │
          HTTP Server        Main Goroutine
                │                │
        ListenAndServe()    <-quit
                │                │
        incoming requests    waiting...

Ab users requests bhej rahe hain:

Client A ──────> Transfer API
Client B ──────> Wallet API
Client C ──────> Transaction History

Server unhe process kar raha hai.

Suddenly deployment/restart hota hai:
                  OS
                   │
                SIGTERM
                   │
                   ▼
             Main Goroutine
                   │
              signal received
                   │
                   ▼
          "Okay, shutdown time."

Main bolta hai:

"Server, naye requests ab mat lena."

             server.Shutdown(ctx)
                     │
                     ▼
             Stop accepting new
                requests

Ab:

New Client ──────X──────> Server
                    rejected/not accepted

Lekin jo requests already chal rahi hain, unko hum kill nahi karte:

Transfer A ───────────────→ finish ✓
Wallet B ─────────────────→ finish ✓
History C ────────────────→ finish ✓

Server wait karta hai.

Aur hum usko bolte hain:

"Main maximum 10 seconds wait karunga."

                 Shutdown
                    │
                    ▼
              ┌───────────┐
              │ 10 seconds│
              └───────────┘
                    │
       ┌────────────┴────────────┐
       │                         │
   work finishes             still running
       │                         │
       ▼                         ▼
      ✓                    timeout reached

Agar sab 10 seconds ke andar finish:

Existing requests finish
          ↓
HTTP server shutdown
          ↓
scheduler.Stop()
          ↓
Redis close
          ↓
DB close
          ↓
main() returns
          ↓
Process exits
Final mental model
                 SIGTERM
                    │
                    ▼
             Main goroutine
                    │
                    ▼
          server.Shutdown(ctx)
                    │
                    ▼
          ┌───────────────────┐
          │ No NEW requests   │
          └─────────┬─────────┘
                    │
                    ▼
          Existing requests
             finish normally
                    │
                    ▼
             ≤ 10 seconds
                    │
             ┌──────┴──────┐
             │             │
          finished       timeout
             │             │
             ▼             ▼
        clean shutdown   stop waiting
             │
             ▼
      scheduler.Stop()
             │
             ▼
        Redis.Close()
             │
             ▼
          DB.Close()
             │
             ▼
        main() returns
             │
             ▼
       Process terminates

One-line definition yaad rakh:

Graceful shutdown = naye kaam ko rokna + already running kaam ko safely complete karne ka limited time dena + resources ko cleanly close karke process exit karna.

## how to do it
Step 1 — Server object banana

Abhi tak tumhare paas Gin router hai:

r := gin.Default()

r ke andar tumhari saari routes/middleware hain.

Lekin humein ab ek HTTP server banana hai jo is router ko actually serve karega.

Isliye next line:

server := &http.Server{
Socho:
r
│
└── "Mere paas routes hain"

http.Server
│
└── "Main in routes ko network par serve karunga"


// --------------------------------
	// HTTP Server
	// --------------------------------
	//till now we had all the routes and middlewares and handler in r which is gin.Default() now we will create a http.Server and pass r as the handler to it
	// so we can serve these routers
	server := &http.Server{
		Addr:    ":" + cfg.HTTP.Port, //configuration se port le rahe hai
		Handler: r,                   //gin.Default() is a handler which will handle all the requests coming to the server so we are passing it to the server i.e r:=gin.Default() and then passing it to the server as a handler
	}
	//now our server is ready to serve the requests coming to the port specified in the configuration file and it will handle the requests using the routes and middlewares we have defined in r
	//normally we would call server.ListenAndServe() to start the server but it will block the main thread and we will not be able to listen for shutdown signals so we will start the server in a goroutine so that it does not block the main thread and we can listen for shutdown signals in the main thread
	//Phir main goroutine SIGTERM ka wait kaise karegi? Isliye server ko alag goroutine mein start karenge.
	//hamen server ko alag goroutine mein start karna hoga taki main goroutine SIGTERM ka wait kar sake aur server ko gracefully shutdown kar sake.
	//for that hum listen and serve ko go func () mein wrap karenge taki ye alag goroutine mein run ho sake aur main goroutine SIGTERM ka wait kar sake.

	// Start server
	go func() { //Is function ko concurrently ek naye goroutine mein chala do.

		logger.Log.Info(
			"server running on " + cfg.HTTP.Port,
		)
		/*
			Kyuki ListenAndServe() error return kar sakta hai.

			err := server.ListenAndServe()

			hum check karna chahte hain:

			"Server unexpectedly fail hua kya?"

			Lekin yahan ek important special case hai: graceful shutdown ke waqt ListenAndServe() normally http.ErrServerClosed return karta hai.
			Lekin graceful shutdown mein ek interesting cheez hoti hai.

			Jab hum baad mein:

			server.Shutdown(ctx)

			call karenge, ListenAndServe() normally ye return karega:Mujhe shutdown kar diya gaya hai, isliye main ListenAndServe() se return kar raha hoon

			http.ErrServerClosed
			Isliye next condition mein hum check karenge:

			err != http.ErrServerClosed

			Taaki normal graceful shutdown ko hum server failure na samjhein.
		*/
		if err := server.ListenAndServe(); err != nil &&
			err != http.ErrServerClosed {

			logger.Log.Error(
				"server failed",
				"error",
				err,
			)
		}
	}()
	/*
	   	//uper  server goroutine ka kaam complete ho jayega.
	   	// --------------------------------
	   	// Wait for shutdown signal
	   	// --------------------------------
	   	//ab hum main goroutine par hain yahan asli graceful shutdown story start hoti hai: SIGTERM ka wait.
	   	// 	Ab main goroutine ka kaam hai:

	   	// "Server ko kab shutdown karna hai?"

	   	// Iske liye OS ke signals sunne padenge.

	   	//Signal receive karne ke liye ek channel

	   Pehle samjho channel kyun?

	   Tumhare paas:

	   Server goroutine
	          │
	          │
	          │  server running...
	          │
	          ▼

	   Main goroutine
	          │
	          │
	          │  shutdown signal ka wait karegi

	   OS jab:

	   SIGTERM

	   bhejega, humein kisi mechanism se ye information main goroutine tak pahunchani hai.

	   Channel us information ko receive karne ka raasta hai.

	   OS
	    │
	    │ SIGTERM
	    ▼
	   signal system
	    │
	    ▼
	   quit channel
	    │
	    ▼
	   main goroutine

	   So:

	   quit := make(chan os.Signal, 1)

	   means:

	   "Ek channel bana do jisme OS signals receive kar sakein."

	   1 kya hai?
	   make(chan os.Signal, 1)

	   mein 1 channel ki buffer capacity hai.

	   Abhi usko deep mein jaane ki zarurat nahi. Bas yaad rakho:

	   quit
	    ↓
	   signal receive karne wala channel

	*/
	quit := make(chan os.Signal, 1)
	//signal.Notify ka kaam hai OS signals ko kisi Go channel tak forward karna.
	signal.Notify(
		quit, //Jo signal receive ho, woh quit channel mein bhejna."
		//Ab signal.Notify ko batana hai:"Kaunse OS signals mujhe listen karne hain?
		os.Interrupt,    // SIGINT → Ctrl+C,Ye basically Ctrl+C wala signal hai PASS KARTA HAI +Jab Ctrl+C / interrupt signal aaye, usko quit channel mein bhej dena.
		syscall.SIGTERM, //Lekin production mein application ko sirf Ctrl+C se shutdown nahi kiya jaata.Ye commonly Docker/Kubernetes/process managers deployment ya termination ke time bhejte hain.
		//Ctrl+C aaye ya SIGTERM aaye, dono situations mein quit channel ko inform karna
	)
	//Ye main goroutine ko wait karayegi.
	//Ye line signal receive hone tak main goroutine ko block karti hai
	//Signal receive hone ke baad hi ye line execute hogi aur main goroutine aage badhegi.
	<-quit

	logger.Log.Info("shutdown signal received")

	// --------------------------------
	// Graceful shutdown
	// --------------------------------
	//Iske baad hum actual graceful shutdown par aayenge:
	// 	Agar hum seedha:

	// server.Shutdown(...)

	// kar dein, toh server ko pata chalega ki shutdown karna hai, but humein usko ye bhi batana hai ki kitni der tak existing requests ke liye wait karna hai.
	// 	Aur ye do values return karta hai:
	// ctx
	// cancel
	// ctx → deadline ki information
	// cancel → context ko manually cancel karne ka function
	// Isliye humein context banana hai.

	// 	context.WithTimeout() ko pehli cheez chahiye: parent context.

	// Abhi hamare paas koi request context nahi hai. Ye shutdown poori application-level operation hai, kisi particular HTTP request ka part nahi.

	// Isliye base context lenge:

	// context.Background(),
	// context.Background() kya hai?

	// Simple:

	// "Ek empty/base context de do jiske upar hum apna timeout laga sakein."
	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	// 	Lekin cancel ko immediately use kyun nahi kiya?

	// Because WithTimeout() internally timer/resource create karta hai.

	// Jab hum context ka kaam complete kar dein, cancel() call karke us resource ko release karna good practice hai.

	// Isliye next line:

	// defer cancel()

	// Meaning:

	// "Jab main() ka ye function scope finish ho, context ko bhi cleanup/cancel kar dena."
	defer cancel()
// server.Shutdown(ctx)

// Ye actual graceful shutdown request hai.

// Server ko bol rahe ho:

// "Bhai, ab shutdown ho ja. Naye requests accept mat kar. Jo requests already chal rahi hain unko finish hone do. Lekin ctx ki deadline ke andar."
	if err := server.Shutdown(ctx); err != nil {
		logger.Log.Error(
			"server forced to shutdown",
			"error",
			err,
		)
	}

	logger.Log.Info("server stopped")
}

# buffered channel and non-biffered channel
Without buffer:

signal ─────→ channel
               │
               └── receiver ready hona chahiye


With buffer 1:

signal ─────→ [ SIGTERM ]
                   │
                   │
              receiver baad mein
              aa sakta hai

What if buffer 0 hota?

Agar:

quit := make(chan os.Signal)

to channel unbuffered hota.

signal.Notify ko signal deliver karne ke liye receiver ready hona important hota.

Buffered 1 ka benefit hai ki ek signal temporarily hold ho sakta hai, even if main goroutine us exact instant par receive karne ke liye ready na ho.

--why 1 Because humein sirf ek shutdown signal ki zarurat hai.
only sigterm so channel ki buffer  capacity=1 rakhi maine.



# database indexing
indexing of database is done to speed up the read but here we have to tradeoff between read and write speed
although it speed up the read but slow down the write rate as the same column data being updates twice * numbers of indexes

--when to use indexing: when a col is being used frequently in joins,filters,search and order then we index the following column.

there are two way of naming the index
1.explicit way : here we keep the name of the index by myself e.g index xyz_klm(col_name)
2.Implicit way: here sql assign the name to the indexes e.g index(col_name)

## how to lock a row
FOR UPDATE in query says
Selected row ko current transaction ke liye lock karo.

FOR UPDATE kya karta hai?

Ab aata hai concurrency problem.

Suppose user double-click karta hai:

Request A
Request B

Both send:

OTP = 123456

Without locking, potentially:

Request A              Request B
    │                      │
    ▼                      ▼
read used=false       read used=false
    │                      │
    ▼                      ▼
both think OTP valid

Dono same OTP consume kar sakte hain.

It doesn't mean nobody can read the row at all.

It means conflicting operations/locks on that row can be blocked until your transaction: commit or rollback

concurrency story

Initial state:

OTP:
used = false

Request A:

BEGIN
 ↓
SELECT ... FOR UPDATE
 ↓
row locked 🔒

Request B:

BEGIN
 ↓
SELECT ... FOR UPDATE
 ↓
WAIT ⏳

Request A:

UPDATE otp
SET used = true

then:

UPDATE users
SET email_verified = true

then:

COMMIT

Lock released:

🔓

Request B continues.

But now:

used = true

so B's query no longer finds a valid OTP.

Therefore:

Request A → success ✓
Request B → invalid OTP ❌

This is the concurrency protection.




Normal
GetActiveOTP(...)

internally:

r.db.QueryRowContext(...)

and:

MarkOTPAsUsed(...)

internally:

r.db.ExecContext(...)

Use when you don't need multiple operations to be atomic.

Transactional
GetActiveOTPTx(...)

internally:

tx.QueryRowContext(...)

with:

FOR UPDATE

and:

MarkOTPAsUsedTx(...)

internally:

tx.ExecContext(...)

Use when this operation is part of a larger transaction.
##  When should you NOT use a transaction?

Don't think:

"Every DB operation should use a transaction."

Instead:

"Use a transaction when multiple operations need to behave as one atomic unit."

Example — no explicit transaction needed
Get user profile

One SELECT:

db.QueryRowContext(...)

Fine.

Example — transaction needed

Wallet transfer:

Debit wallet A
Credit wallet B
Create ledger entry

These should succeed together:

BEGIN
 ↓
debit A
 ↓
credit B
 ↓
ledger entry
 ↓
COMMIT
Another example — registration

Tumhara existing:

Create User
   +
Create Wallet

should be atomic.

That's why you already have:

CreateTx(ctx, user, tx)
CreateTx(ctx, wallet, tx)

Excellent use case.
only sigterm so channel ki buffer  capacity=1 rakhi maine.
