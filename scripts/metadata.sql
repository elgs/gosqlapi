-- @label: metadata
SELECT
!remote_addr! as "REMOTE_ADDRESS",
!host! as "HOST",
!method! as "METHOD",
!path! as "PATH",
!query! as "QUERY",
!user_agent! as "USER_AGENT",
!referer! as "REFERER",
!accept! as "ACCEPT",
!AUThorization! as "AUTHORIZATION"
FROM TEST_GOSQLAPI;