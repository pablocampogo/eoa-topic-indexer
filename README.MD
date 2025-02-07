# EOA TOPIC INDEXER

This is a simple implementation of an indexer of an EOA ethereum address in sepolia of a given topic.

## Prerequisites

Golang 1.22 or higher

## How to run

Run and build the project with Golang comments and fill the .env file before running using the .env.example as guide:

```
MAX_DURATION=5000
PORT=8080
DB_PATH=l1_data.db
START_BLOCK=0
END_BLOCK=0
BLOCK_TAG=latest
DURATION_CHECK=5000
ADDRESS={YOUR_ADDRESS}
TOPIC={YOUR_TOPIC}
CONCURRENCY=100
BLOCK_TIME=20
RPC_TIMEOUT=60
MAX_ROUNDS=3
ENDPOINTS={YPUR_ENDPOINTS}
```
Endpoints should be separated by a comma.

The indexer has two modes of running:
  - **Full Mode:**
    - You specify a tag (`latest`, `safe`, or `finalized`).
    - It indexes until that tag and keeps updating as the service runs.
    - If the service is turned off, it continues indexing from where it left off when restarted.
  
  - **Range Mode:**
    - You specify the start and end block.
    - It indexes only within that specified range.


## Endpoints

It runs on the specified HTTP port and the ports are:
  - **`/data`**: Retrieves paginated data in JSON format with the inputs `startIndex` and `limit`.
  - **`/health`**: Returns `200 OK` if the service is running.
  - **`/last-block-indexed`**: Returns the last block that is indexed in the database.
