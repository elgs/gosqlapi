SELECT
    TARGET_DATABASE AS target_database,
    TARGET_OBJECTS AS target_objects,
    READ_PRIVATE AS read_private,
    WRITE_PRIVATE AS write_private,
    EXEC_PRIVATE AS exec_private,
    ALLOWED_ORIGINS AS allowed_origins
FROM TOKENS
WHERE TOKEN = ?token?;