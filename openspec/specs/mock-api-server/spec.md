# mock-api-server Specification

## Purpose
Defines the stateful fake of the RevenueCat API v2 catalog that the provider is verified against,
so end-to-end tests can run offline and deterministically without a live RevenueCat account.

## Requirements

### Requirement: Mock implements catalog CRUD semantics
The mock SHALL implement create, read, update, delete and list for apps, products, entitlements,
offerings and packages, holding state in memory for the lifetime of the process. A created object
SHALL be assigned a unique identifier and a creation timestamp, and SHALL be readable at its
identifier immediately afterwards.

#### Scenario: Created object is readable
- **WHEN** a client creates an entitlement and then reads it by the returned identifier
- **THEN** the mock returns the same object, including the identifier and creation timestamp it assigned

#### Scenario: Identifiers are unique
- **WHEN** two objects of the same type are created
- **THEN** they receive different identifiers

#### Scenario: Update changes only the supplied fields
- **WHEN** a client updates an entitlement supplying only a display name
- **THEN** the stored entitlement's display name changes and its lookup key is left as it was
- **AND** the response carries the full updated object

#### Scenario: Deleted object is gone
- **WHEN** a client deletes an offering and then reads it
- **THEN** the mock responds `404`

#### Scenario: Absent object reads as not found
- **WHEN** a client reads an identifier that was never created
- **THEN** the mock responds `404` with a structured error body

### Requirement: Mock scopes objects to their parents
The mock SHALL reject operations that address an object through the wrong parent, so a test cannot
pass by accident when the provider sends a mismatched project or offering.

#### Scenario: Wrong project
- **WHEN** a client reads an entitlement using a project identifier other than the one it was created under
- **THEN** the mock responds `404`

#### Scenario: Package listed under its offering
- **WHEN** packages are created under two different offerings and a client lists one offering's packages
- **THEN** only that offering's packages are returned

#### Scenario: App from another project
- **WHEN** a client creates a product, or creates or updates a webhook integration, naming an `app_id` that belongs to a different project
- **THEN** the mock responds `400`, while a webhook's `app_id` of `null` is still accepted

### Requirement: Mock paginates list responses
The mock SHALL return list responses in the documented envelope, with an `items` array and a
`next_page` cursor when more items remain. The page size SHALL be configurable so pagination can be
exercised with a small number of objects.

#### Scenario: Single page
- **WHEN** fewer objects exist than the page size
- **THEN** the response carries all of them and no next-page cursor

#### Scenario: Multiple pages
- **WHEN** more objects exist than the page size
- **THEN** the first response carries a next-page cursor, and following it returns the remaining
  objects without repeating any

### Requirement: Mock tracks product attachments
The mock SHALL record which products are attached to each entitlement and package, SHALL apply
attach and detach actions to that record, and SHALL return the current record when the attached
products are listed. Attaching an already-attached product SHALL update its eligibility criteria
rather than duplicating it.

#### Scenario: Attach then list
- **WHEN** two products are attached to an entitlement and its products are listed
- **THEN** both products are returned

#### Scenario: Detach removes only the named products
- **WHEN** one of two attached products is detached
- **THEN** listing returns only the other product

#### Scenario: Re-attaching updates eligibility criteria
- **WHEN** a product already attached to a package is attached again with a different eligibility criteria
- **THEN** listing returns that product once, carrying the new criteria

### Requirement: Mock authenticates requests
The mock SHALL reject any request that does not carry a bearer token with `401`, so a test cannot
pass while the provider omits or malforms its credential.

#### Scenario: Missing credential
- **WHEN** a request arrives without an `Authorization` header
- **THEN** the mock responds `401` and does not modify its state

#### Scenario: Wrong authentication scheme
- **WHEN** a request arrives with an `Authorization` header that is not a bearer token
- **THEN** the mock responds `401`

### Requirement: Mock can inject faults on demand
The mock SHALL support being configured to fail a given number of requests with `429` before
serving normally, so retry behavior can be exercised end to end.

#### Scenario: Rate limit then success
- **WHEN** the mock is configured to rate-limit the first request and a client makes two requests
- **THEN** the first receives `429` with a `Retry-After` header and the second succeeds

#### Scenario: Fault injection is off by default
- **WHEN** the mock is started with no fault configuration
- **THEN** every well-formed request is served normally

### Requirement: Mock is runnable as a standalone server
The mock SHALL be runnable as a standalone binary that listens on a configurable address and can be
seeded with a project of a known identifier, so it can back a container-based end-to-end run.

#### Scenario: Seeded project is readable
- **WHEN** the server is started with a seeded project identifier and a client reads that project
- **THEN** the project is returned

#### Scenario: Health check
- **WHEN** a client requests the server's health endpoint
- **THEN** the server responds `200` without requiring authentication
