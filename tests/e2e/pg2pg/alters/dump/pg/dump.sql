create type "fancyEnum" as enum ('val1', 'val2');
create table __test
(
    id   int,
    "Val1" int,
    val2 varchar not null default 'foo',
    "FancyEnum" "fancyEnum",
    primary key (id)
);

insert into __test (id, "Val1", val2, "FancyEnum")
values (1, 1, 'a', 'val1'),
       (2, 2, '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', 'val2')
        -- long string value in val2 - for TOAST testing. It should be random, bcs 'to TOAST or not to TOAST' decision happens after compression of values
