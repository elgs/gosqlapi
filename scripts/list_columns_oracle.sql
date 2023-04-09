SELECT 
utc.TABLE_NAME, 
utc.COLUMN_NAME,
utc.DATA_TYPE,
utc.DATA_LENGTH,
utc.DATA_PRECISION,
utc.DATA_SCALE,
utc.NULLABLE,
case when pk.column_name = utc.COLUMN_NAME then 'Y' else 'N' end as is_pk
FROM user_tab_columns utc 

left outer join (
SELECT cols.table_name, cols.column_name
FROM all_constraints cons, all_cons_columns cols
WHERE cols.table_name = ?table_name?
AND cons.constraint_type = 'P'
AND cons.constraint_name = cols.constraint_name
AND cons.owner = cols.owner
ORDER BY cols.table_name, cols.position
) pk on utc.table_name=pk.table_name
and utc.COLUMN_NAME=pk.column_name

WHERE utc.table_name=?table_name?;