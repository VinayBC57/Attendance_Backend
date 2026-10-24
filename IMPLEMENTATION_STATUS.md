# Backend Implementation Status

## Feature flow status

| Flow | Status | Notes |
|---|---|---|
| Health endpoint | Complete | `GET /health` |
| Admin login | Complete | Seeded `ADMIN-001` account |
| Employee login | Complete | Temporary password and JWT issuance |
| First-login password change | Complete | Enforces password policy and writes `password_history` |
| CORS for local web UI | Complete | Permissive origin reflection for local development |
| Swagger documentation | Complete | Available at `/swagger/index.html` |
| Create employee | Complete | Generates ID and temporary password |
| List employees | Pending | Admin needs a paginated/searchable employee endpoint |
| Employee details | Pending | Admin needs to fetch one employee profile |
| Edit employee details | Pending | Admin needs update validation and persistence |
| Admin password reset request | Pending | Generate a new temporary password and require first-login change |
| Current-user profile | Pending | UI session refresh depends on `GET /api/v1/auth/me` |
| JWT authorization middleware | Pending | Routes are not currently role-protected by middleware |
| Employee attendance check-in/out | Pending | Database model exists; HTTP endpoints are absent |
| Admin attendance dashboard | Pending | No summary or attendance-list endpoint yet |
| Attendance reporting | Pending | No report endpoint yet |

## Employee administration flow

### Completed

1. Admin authenticates with `POST /api/v1/auth/login`.
2. Admin sends employee details to `POST /api/v1/admin/employees`.
3. Backend creates `User` and `Employee` records.
4. Backend generates a temporary password and returns it once.
5. Employee signs in and receives `require_password_change: true`.
6. Employee changes the password through `POST /api/v1/auth/change-password`.
7. Backend updates password state and records `change_type: "first_login"`.

### Pending for points 1-3

- `GET /api/v1/admin/employees`: searchable employee list.
- `GET /api/v1/admin/employees/:employee_id`: employee details.
- `PUT /api/v1/admin/employees/:employee_id`: update editable identity and profile fields.
- `POST /api/v1/admin/employees/:employee_id/password-reset`: generate and store a new temporary password; set `is_first_login_required=true`; record an audit entry.

The reset endpoint must not return password history or password hashes. It should return the new temporary password once so the admin can transmit it securely.

## API expectations

- Every admin endpoint must require a valid JWT and verify `role == "admin"`.
- List endpoint should support `search`, `page`, and `page_size` query parameters.
- Employee IDs must remain immutable.
- Email and phone uniqueness must be checked before updates.
- Password resets must use the existing password policy and bcrypt storage behavior.
- Mutation endpoints should use database transactions where employee and user records change together.

## Deferred work

Attendance dashboard, attendance history, reports, account deactivation, and detailed audit browsing are intentionally outside the current employee-admin implementation slice.
