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