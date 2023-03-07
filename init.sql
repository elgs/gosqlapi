drop TABLE IF EXISTS test_table;

create TABLE test_table(  
    ID INTEGER NOT NULL PRIMARY KEY,
    NAME VARCHAR(50)
);

insert INTO test_table (ID, NAME) VALUES (1, 'Alpha');

insert INTO test_table (ID, NAME) VALUES (2, 'Beta');

insert INTO test_table (ID, NAME) VALUES (3, 'Gamma');


-- @label: data
SELECT * FROM test_table WHERE ID > ?low? AND ID < ?high?;