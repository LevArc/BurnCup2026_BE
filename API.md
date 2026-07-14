# BurnCup Backend API Documentation

## Overview

Base URL:

```text
http://localhost:8080/api
```

The API is a Gin-based REST service backed by PostgreSQL.

## Authentication

Protected routes require a JWT in the `Authorization` header:

```http
Authorization: Bearer <jwt>
```

The token must be signed with the backend `JWT_SECRET_KEY` value.

## Common Response Models

### Competition

Returned by competition listing/detail endpoints.

Fields include:

- `id`
- `name`
- `description`
- `category`
- `imageUrl`
- `bookletUrl`
- `paidMessage`
- `registrationStartDate`
- `registrationEndDate`
- `competitionStartDate`
- `competitionEndDate`
- `competitionType`
- `venue`
- `registrationfee`
- `maxMembers`
- `minMembers`
- `teamSlot`
- `createdAt`
- `updatedAt`
- `prizes`
- `requirements`
- `rules`

### User

Returned by user profile endpoints.

Fields include:

- `userType`
- `fullName`
- `phoneNumber`
- `email`
- `nim`
- `major`
- `school`

## Public Endpoints

### GET `/`

Health check for the backend server.

Response:

```json
"BurnCup API is running"
```

### GET `/api/public`

Simple public test endpoint.

Response:

```json
{
  "message": "This is a public endpoint"
}
```

### GET `/api/competitions`

Returns all competitions with prizes, requirements, and rules.

Response: array of competition objects.

### GET `/api/competitions/:id`

Returns one competition by ID with prizes, requirements, and rules.

Path params:

- `id`: competition ID

Response: competition object.

### GET `/api/get-remaining-team-slot/:competitionId`

Returns remaining paid team slots for a competition.

Path params:

- `competitionId`: competition ID

Response:

```json
{
  "remainingSlots": 5
}
```

### POST `/api/midtrans/hook`

Midtrans payment webhook handler.

Request body:

- `transaction_status`
- `status_code`
- `transaction_id`
- `order_id`
- `payment_type`
- `gross_amount`
- `fraud_status`
- `signature_key`

Notes:

- Only `settlement` and `capture` statuses are processed.
- The signature is validated before any database update.

### GET `/api/ping-is-paid-team-slot/:teamId`

Returns whether a team is paid and how many slots remain in the competition.

Path params:

- `teamId`: registered competition ID

Response:

```json
{
  "isPaid": false,
  "remainingSlots": 3
}
```

### GET `/api/qr/:value`

Generates a QR code PNG for the given value.

Path params:

- `value`: QR payload

Response:

- `image/png`

## Protected Endpoints

All endpoints in this section require a valid Bearer JWT.

### GET `/api/protected`

Simple authorization check endpoint.

Response:

```json
{
  "message": "You are authorized to access this protected endpoint"
}
```

### GET `/api/protected/get-current-user`

Returns the current user profile identified by the JWT email claim.

Response: user object.

### POST `/api/protected/create-update-user-profile`

Creates or updates the current user profile.

Request body:

- `userType`
- `fullName`
- `phoneNumber`
- `nim` optional
- `major` optional
- `school` optional

The `email` field is taken from the JWT and not from the request body.

Response: created/updated user object.

### POST `/api/protected/create-team`

Creates a new team for a competition.

Request body:

- `competitionId` required
- `teamName` required

Response:

```json
{
  "teamId": "...",
  "teamCode": "..."
}
```

### GET `/api/protected/get-teams`

Returns all teams the current user belongs to.

Response: array of team objects with competition, members, and team leader.

### POST `/api/protected/join-team`

Joins an existing team by code.

Request body:

- `teamCode` required
- `competitionId` required

Response:

```json
{
  "message": "Successfully joined the team"
}
```

### DELETE `/api/protected/delete-team-member`

Removes a member from a team. Only the team leader can call this, and only before the team is paid.

Request body:

- `teamId` required
- `memberEmail` required

Response:

```json
{
  "message": "Team member removed successfully",
  "teamId": "...",
  "removedMember": "..."
}
```

### GET `/api/protected/get-qr-link/:teamCode`

Creates or reuses a Midtrans QR payment link for a team.

Path params:

- `teamCode`: team code

Response:

```json
{
  "qrLink": "https://...",
  "expiryTime": "2026-07-12T10:00:00Z"
}
```

### GET `/api/protected/admin-basic-info`

Returns summary counts for the admin dashboard.

Response fields:

- `totalUsers`
- `binusianUsers`
- `smaUsers`
- `otherUsers`
- `totalTeams`
- `activeCompetitions`
- `upcomingEvents`
- `totalParticipants`
- `categoryCount`

### GET `/api/protected/admin-competitions-statistics`

Returns per-competition statistics for the admin dashboard.

Response fields for each item:

- `id`
- `name`
- `category`
- `totalTeams`
- `totalParticipants`
- `paidTeams`
- `pendingTeams`
- `registrationFee`
- `competitionType`

### GET `/api/protected/admin-all-teams`

Returns all registered teams with competition, members, and team leader details.

Response: array of team objects.

### POST `/api/protected/admin-add-competition`

Creates a new competition.

Request body:

- `name` required
- `description` required
- `category` required
- `imageUrl` required
- `bookletUrl` required
- `paidMessage` required
- `registrationStartDate` required
- `registrationEndDate` required
- `competitionStartDate` required
- `competitionEndDate` required
- `competitionType` required
- `venue` required
- `registrationfee` required
- `prizes` optional array
- `requirements` optional array
- `rules` optional array
- `maxMembers` optional
- `minMembers` optional
- `teamSlot` required

Response:

```json
{
  "message": "Competition created successfully",
  "competitionId": "..."
}
```

### POST `/api/protected/admin-update-competition/:id`

Updates an existing competition and replaces its prizes, requirements, and rules.

Path params:

- `id`: competition ID

Request body: same shape as add competition, but fields are optional in the handler.

Response:

```json
{
  "message": "Competition updated successfully",
  "competitionId": "..."
}
```

### DELETE `/api/protected/admin-delete-competition/:id`

Deletes a competition if it has no registered teams.

Path params:

- `id`: competition ID

Response:

```json
{
  "message": "Competition deleted successfully",
  "competitionId": "..."
}
```

### DELETE `/api/protected/admin-delete-team/:teamCode`

Deletes an unpaid team and its members.

Path params:

- `teamCode`: team code

Response:

```json
{
  "message": "Team deleted successfully",
  "teamCode": "...",
  "teamName": "...",
  "competitionId": "..."
}
```

## Example Requests

### Create or update a user profile

```bash
curl -X POST http://localhost:8080/api/protected/create-update-user-profile \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d '{
    "userType": "Binusian",
    "fullName": "Jane Doe",
    "phoneNumber": "08123456789",
    "nim": "2412345678",
    "major": "Computer Science",
    "school": "BINUS University"
  }'
```

### Create a team

```bash
curl -X POST http://localhost:8080/api/protected/create-team \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d '{
    "competitionId": "competition-id",
    "teamName": "Alpha Team"
  }'
```

### Join a team

```bash
curl -X POST http://localhost:8080/api/protected/join-team \
  -H "Authorization: Bearer <jwt>" \
  -H "Content-Type: application/json" \
  -d '{
    "teamCode": "ABCD1234",
    "competitionId": "competition-id"
  }'
```

## Notes

- Dates are stored and returned as strings.
- `competitionfee` and `registrationfee` are the field names used by the current backend structs and responses.
- Some admin and team endpoints rely on database state and may return partial results if related records are missing.