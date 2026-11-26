# Test with PostgreSQL and a controlled AI provider

Use Testcontainers for Go to run integration tests against PostgreSQL locally and in CI. Apply the same Tern application migrations and `rivermigrate` queue migrations used by the local demo to exercise transactional behavior, including import and job scheduling and competing reconciliation confirmations.

Use a test double at the candidate ranking interface to exercise valid rankings, invalid responses, and provider failures. This keeps integration tests reproducible and independent of external AI credentials.
