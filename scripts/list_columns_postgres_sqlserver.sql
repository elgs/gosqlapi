SELECT col.*, con.constraint_type FROM INFORMATION_SCHEMA.COLUMNS col 
LEFT OUTER JOIN (
select con.*,usage.column_name from INFORMATION_SCHEMA.TABLE_CONSTRAINTS con
inner join information_schema.key_column_usage usage
    on con.table_name = usage.table_name and con.table_schema = usage.table_schema and con.constraint_name = usage.constraint_name
where con.constraint_type='PRIMARY KEY' 
) con on col.TABLE_SCHEMA=con.TABLE_SCHEMA and col.TABLE_NAME=con.TABLE_NAME and col.column_name=con.column_name
WHERE col.TABLE_NAME=?table_name?;