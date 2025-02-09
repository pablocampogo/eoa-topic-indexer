# EOA TOPIC INDEXER

This is a simple implementation of an indexer of an EOA ethereum address in sepolia of a given topic.

## Prerequisites

Golang 1.22 or higher

## Modes

The indexer has two modes of running:
  - **Full Mode:**
    - You specify a tag (`latest`, `safe`, or `finalized`).
    - It indexes until that tag and keeps updating as the service runs.
    - If the service is turned off, it continues indexing from where it left off when restarted.
  
  - **Range Mode:**
    - You specify the start and end block.
    - It indexes only within that specified range.

## How to run

1. Before everything you should fill the .env file taking as example the .env.example one:

```
# MANDATORY CONFIGURATION

# ADDRESS specifies the Ethereum address to fetch data from, it must be set
ADDRESS={YOUR_ADDRESS}
# TOPIC specifies the topic hash to fetch data from, it must be set
TOPIC={YOUR_TOPIC}
# ENDPOINTS is a list of RPC endpoints to be used for making requests, they must be set separated by commas
ENDPOINTS={YOUR_ENDPOINTS}

# OPTIONAL CONFIGURATION

# Processing

# START_BLOCK represents the starting block for range-based processing, defaults to 0.
START_BLOCK=0
# END_BLOCK represents the ending block for range-based processing, defaults to 0.
END_BLOCK=0
# BLOCK_TAG represents the block tag ("latest", "safe" or "finalized") for full processing, defaults to "latest".
BLOCK_TAG=latest

# Server

# PORT defines the HTTP server port, defaults to 8080
PORT=8080

# DB

# DB_PATH defines the file path for the BoltDB database, defaults to l1_data.db
DB_PATH=l1_data.db

# Execution intervals

# CYCLE_INTERVAL defines the maximum duration in MS for the orchestrator ticker interval, defaults to 5000 ms (5 seconds)
CYCLE_INTERVAL=5000
# SYNC_INTERVAL defines the maximum duration in MS for the synchronization ticker interval, defaults to 5000 ms (5 seconds)
SYNC_INTERVAL=5000

# Performance & Limits

# CONCURRENCY defines the number of concurrent fetch operations, defaults to 100
CONCURRENCY=100
# BLOCK_TIME is the duration in seconds to block a client after a rate limit or timeout error, defaults to 20 seconds
BLOCK_TIME=20
# RPC_TIMEOUT is the maximum time in seconds to wait for a response from the RPC endpoint, defaults to 60 seconds
RPC_TIMEOUT=60
# MAX_ROUNDS is the maximum number of retry rounds allowed trying to do the RPC calls, defaults to 3
MAX_ROUNDS=3
```
**Important note**: for better perfomance, please use Alchemy RPC endpoints.

2. Build the project:

```sh
make build
```

3. Run the project:

```sh
make start
```

## API endpoints

### `/data`
Retrieves indexed blockchain data in a paginated format.

#### **Query Parameters**
- `startIndex` (integer, optional, default: `0`): The index from which to start fetching data.
- `limit` (integer, optional, default: `10`): The maximum number of records to return in a single response.

#### **Response**
Returns a JSON object containing:
- `data`: An array of indexed blockchain records.
- `nextPage`: A URL to fetch the next page of results, if more data is available.

#### **Example Request**
```bash
GET /data?startIndex=0&limit=50
```

#### **Example Response**
```json
{
  "data": [
    {
      "index": 0,
      "l1InfoRoot": "0xabc123...",
      "blockTime": 1700000000,
      "blockParentHash": "0xdef456..."
    },
    {
      "index": 1,
      "l1InfoRoot": "0xghi789...",
      "blockTime": 1700000001,
      "blockParentHash": "0xjkl012..."
    }
  ],
  "nextPage": "/data?startIndex=50&limit=50"
}
```

### `/health`

Returns `200 OK` if the service is running.

### `/last-block-indexed`

Returns the last block that is indexed in the database.
