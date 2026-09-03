create table otp_codes(
    id varchar(36) primary key,
    user_id varchar(36) not null,
    code varchar(6) not null, 
    -- email verification code or password reset code
    type varchar(32) not null,
    expires_at timestamp not null,
    used boolean not null default false,
    created_at timestamp not null default current_timestamp,
    foreign key (user_id) references users(id) on delete cascade
)