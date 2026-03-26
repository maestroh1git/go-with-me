# API Reference

### Auth endpoints

| Method | Path | Auth required | Rate limit |
|---|---|---|---|
| POST | `/auth/login` | None | 30/min |
| POST | `/auth/verify-totp` | MFA token | — |
| POST | `/auth/totp/setup` | TOTP setup token | — |
| POST | `/auth/totp/confirm` | TOTP setup token | — |
| POST | `/auth/forgot-password` | None | 10/min |
| POST | `/auth/verify-reset-email` | None | 10/min |
| POST | `/auth/complete-password-reset` | Password reset token | — |
| POST | `/auth/accept-invite` | None | 20/min |
| GET | `/auth/me` | Session token | — |

### User endpoints

| Method | Path | Auth required | Roles | Rate limit |
|---|---|---|---|---|
| GET | `/api/v1/users` | Session | admin, super_admin | 100/min |
| GET | `/api/v1/users/:id` | Session | admin, super_admin, manager | — |
| PATCH | `/api/v1/users/:id` | Session | admin, super_admin | — |
| POST | `/api/v1/users/invite` | Session | admin, super_admin | 20/min |
| DELETE | `/api/v1/users/:id` | Session | super_admin | — |

### Health probes

| Method | Path | Auth required |
|---|---|---|
| GET | `/health` | None |
| GET | `/ready` | None |

---

## Auth flows

### Login flow

```
POST /auth/login (email + password)
  │
  ├── Invalid credentials → 401
  ├── Account locked → 400
  │
  ├── TOTP enrolled:
  │     → { mfa_required: true, mfa_token: "..." }
  │     POST /auth/verify-totp (mfa_token + code)
  │     → { access_token, expires_at }
  │
  └── TOTP not set up (mandatory 2FA):
        → { totp_setup_required: true, setup_token: "..." }
        POST /auth/totp/setup (Bearer: setup_token)
        → { secret, otpauth_url }
        POST /auth/totp/confirm (Bearer: setup_token, secret + code)
        → { access_token, expires_at }
```

### Password reset flow

```
POST /auth/forgot-password (email)
  → Always 200 (anti-enumeration)
  → OTP emailed (logged to console in development)

POST /auth/verify-reset-email (email + otp)
  → { reset_token: "..." }

POST /auth/complete-password-reset (Bearer: reset_token, new_password [+ totp_code])
  → { message: "password updated" }
```

### Invite flow

```
POST /api/v1/users/invite (admin session, email + name + role)
  → { invite_token: "..." }
  [Token delivered to user via email]

POST /auth/accept-invite (invite_token + name + password)
  → { totp_setup_required: true, setup_token: "..." }
  [User then completes TOTP setup flow]
```
