# Transition towards microsevices from monolith
day27: 
OBJECTIVE:
1. treansform our project to monorepo  using go workspace
2.create a new service called api-gateway as single entry point for all the  client requests
3. implement reverse proxy using go standard library to forward traffic from gatway to the targeted microsevices

## why need api gateway
when we split monolith to microsevice each service will run on diffrent ports (e.g., auth on :8081, wallet on :8082)
it's very bad of client or mobile has to remember all the ports 
soln:
/api/v1/users/*->forwarded to user/auth service 
/api/v1/wallet/*->forwarded to wallet service 

step1: migrate to go workspace(monorepo)
we will move current monolith  folder to api gateway and seperate  it into workspace.
In the project  root folder (wallet-microservices/) delete old workspace file if exits then run initalization

# initalize go workspace in the root folder
go work init

# create new folder for api-gateway  and copy old  monolith code  to new  sub-folder




# docker setup for microservice
                    ┌─────────────────────────┐
                    │        HOST MACHINE     │
                    │                         │
                    │   localhost:8080        │
                    │          │              │
                    └──────────┼──────────────┘
                               │
                               ▼
                 ┌──────────────────────────┐
                 │   api-gateway container  │
                 │                          │
                 │ sudowallet-api-gateway-ms│
                 │        :8080             │
                 └────────────┬─────────────┘
                              │
                  sudowallet-network
                              │
        ┌─────────────┬───────┼────────┬─────────────┐
        ▼             ▼       ▼        ▼             ▼
   auth-service  user-service wallet  transaction  payment
     :8081         :8084      :8082      :8086       :8083
                              │
                              ▼
                       ┌────────────┐
                       │   MySQL    │
                       │   :3306    │
                       └────────────┘

                       ┌────────────┐
                       │   Redis    │
                       │   :6379    │
                       └────────────┘

                       ┌────────────┐
                       │  MailHog   │
                       │   :1025    │
                       └────────────┘   

check 1st commit of branch feat/018.api-gateway 

# auth service splittiong plan 
in our microservice the auth service is handles session management and token security

1. clean architecture with repository pattern: database query language has been cleanly seperated from  the service layer by introducing  userRepository and refreshTokenRepository
2.shared modules(clean architecture): genric helper functionilities (database connections,logging,configuration loading,jwt utilities, and error handling ,middlewares) are imported from the shared module  rather then duplicate inside the auth service.
3.effeciency:by seperating auth service ,if login traffic surges we can scale up the auth service conatainer only without duplicating other resources.

## create auth-service folder with clean architecture pattern
auth-service:
    cmd:
    internal:
     auth:
      handler:
      service:
      repository:
      model: