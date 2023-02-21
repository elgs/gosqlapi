drop TABLE IF EXISTS test_table;

create TABLE IF NOT EXISTS test_table(  
    ID INTEGER NOT NULL PRIMARY KEY,
    NAME TEXT
);

insert INTO test_table (ID, NAME) VALUES (1, 'Alpha');

insert INTO test_table (ID, NAME) VALUES (2, 'Beta');

insert INTO test_table (ID, NAME) VALUES (3, 'Gamma');


-- @label: data
SELECT * FROM test_table;