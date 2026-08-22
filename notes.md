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