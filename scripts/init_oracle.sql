drop TABLE TEST_TABLE;
drop TABLE TOKENS;

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
  EXEC_PRIVATE INT NOT NULL,
  ALLOWED_ORIGINS VARCHAR(1000) NOT NULL
);
create INDEX TOKEN_INDEX ON TOKENS (TOKEN);

insert INTO 
TOKENS (ID, TOKEN,        TARGET_DATABASE,  TARGET_OBJECTS,   READ_PRIVATE, WRITE_PRIVATE,  EXEC_PRIVATE, ALLOWED_ORIGINS)
VALUES (1,  '1234567890', 'test_db',        'token_table',    1,            0,              0,            'localhost');

insert INTO 
TOKENS (ID, TOKEN,        TARGET_DATABASE,  TARGET_OBJECTS,   READ_PRIVATE, WRITE_PRIVATE,  EXEC_PRIVATE, ALLOWED_ORIGINS)
VALUES (2,  '0987654321', 'test_db',        'metadata',       0,            0,              1,            'localhost *.example.com');

insert INTO 
TOKENS (ID, TOKEN,        TARGET_DATABASE,  TARGET_OBJECTS,   READ_PRIVATE, WRITE_PRIVATE,  EXEC_PRIVATE, ALLOWED_ORIGINS)
VALUES (3,  'no_access',  'test_db',        '*',              1,            1,              1,            ' ');

insert INTO 
TOKENS (ID, TOKEN,        TARGET_DATABASE,  TARGET_OBJECTS,   READ_PRIVATE, WRITE_PRIVATE,  EXEC_PRIVATE, ALLOWED_ORIGINS)
VALUES (4,  'super',      'test_db',        '*',              1,            1,              1,            '*');