**Directus API Analytics Collection Specification**  
*(Minimal version – single collection only)*

### Collection: `api_request_logs`

**Collection Settings**  
- **Collection**: `api_request_logs`  
- **Icon**: `api`  
- **Color**: `#2563eb`  
- **Note**: Raw API request logs (successful + rejected + rate-limit breaches). This is the only collection.  
- **Display Template**: `{{method}} {{path}} • {{status_code}} • {{user_email}}`

**Fields** (create in this exact order):

| Field Name            | Type          | Required | Interface                  | Display               | Notes |
|-----------------------|---------------|----------|----------------------------|-----------------------|-------|
| `created_at`          | Timestamp     | Yes      | datetime                   | datetime              | Default: `CURRENT_TIMESTAMP` (Directus/MySQL equivalent of `$NOW`), Readonly: Yes |
| `user_email`          | String        | No       | input                      | email                 | Email of the authenticated user (or empty for public/unauth requests) |
| `api_key_id`          | String        | No       | input                      | raw                   | Optional |
| `user_id`             | String        | No       | input                      | raw                   | Optional |
| `ip_address`          | String        | No       | input                      | raw                   | — |
| `user_agent`          | Text          | No       | textarea                   | raw                   | — |
| `method`              | String        | Yes      | select-dropdown            | raw                   | Choices: GET, POST, PUT, DELETE, PATCH, HEAD |
| `path`                | String        | Yes      | input                      | raw                   | — |
| `query_params`        | JSON          | No       | json                       | json                  | — |
| `status_code`         | Integer       | Yes      | input                      | formatted-number      | — |
| `response_time_ms`    | Integer       | No       | input                      | formatted-number      | Suffix: `ms` |
| `is_rate_limit_breach`| Boolean       | No       | boolean                    | boolean               | — |
| `remaining_quota`     | Integer       | No       | input                      | formatted-number      | — |
| `error_message`       | Text          | No       | textarea                   | raw                   | Only for rejected requests (status ≥ 400) |

**After creating fields**:  
Add indexes on: `user_email`, `created_at`, `path`, `status_code`, `is_rate_limit_breach`.  