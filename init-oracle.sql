drop TABLE TEST_TABLE;
drop table TOKENS;

create TABLE TEST_TABLE (
    ID INTEGER NOT NULL PRIMARY KEY,
    NAME VARCHAR(50)
);

insert INTO TEST_TABLE (ID, NAME) VALUES (1, 'Alpha');

insert INTO TEST_TABLE (ID, NAME) VALUES (2, 'Beta');

insert INTO TEST_TABLE (ID, NAME) VALUES (3, 'Gamma');


-- @label: data
SELECT * FROM TEST_TABLE WHERE ID > ?low? AND ID < ?high?;

create TABLE TOKENS ( 
  ID INTEGER NOT NULL PRIMARY KEY,
  TOKEN VARCHAR(255) NOT NULL,
  TARGET_DATABASE VARCHAR(255) NOT NULL,
  TARGET_OBJECTS VARCHAR(255) NOT NULL,
  READ_PRIVATE INT NOT NULL,
  WRITE_PRIVATE INT NOT NULL,
  EXEC_PRIVATE INT NOT NULL
);
create INDEX TOKEN_INDEX ON TOKENS (TOKEN);

insert INTO 
TOKENS (ID, TOKEN,        TARGET_DATABASE,  TARGET_OBJECTS, READ_PRIVATE, WRITE_PRIVATE,  EXEC_PRIVATE)
VALUES (1,  '1234567890', 'test_db',        'token_table',  1,            0,              0);
