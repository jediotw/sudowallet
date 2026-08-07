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