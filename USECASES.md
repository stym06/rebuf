# rebuf — Use Cases

`rebuf` is a lightweight, disk-backed Write-Ahead Log (WAL) for Go. It acts as a durable buffer between your application and any unreliable dependency, ensuring data is never lost during outages.

Below are common scenarios where rebuf fits naturally.

---

## 1. Message Queue Outage Recovery

Your service produces events to Kafka, RabbitMQ, or another broker. If the broker becomes unavailable, events are silently dropped or your service crashes.

**With rebuf:** buffer events to disk while the broker is down, then replay and re-publish them once it recovers.

```go
// Producer writes events to rebuf when the broker is unreachable.
if err := broker.Publish(event); err != nil {
    r.Write(event)
}

// When the broker recovers, replay the buffer.
r.Replay(func(data []byte) error {
    return broker.Publish(data)
})
r.Purge()
```

---

## 2. Database Failover Buffering

Database failovers, maintenance windows, or connection pool exhaustion can cause brief periods of unavailability. Losing writes during these windows is unacceptable for many workloads.

**With rebuf:** capture writes locally and replay them once the database is reachable again.

```go
if err := db.Insert(record); err != nil {
    r.Write(serialize(record))
}

// After failover completes.
r.Replay(func(data []byte) error {
    return db.Insert(deserialize(data))
})
r.Purge()
```

---

## 3. Microservice Outbox Pattern

Service A calls Service B over HTTP or gRPC. If Service B is down, requests fail and data can be lost. The transactional outbox pattern solves this, but often requires a database.

**With rebuf:** use a lightweight, file-based outbox instead. Log payloads locally and deliver them when the downstream service recovers.

```go
// In the request path — fall back to rebuf on failure.
resp, err := httpClient.Post(serviceB, payload)
if err != nil {
    r.Write(payload)
}

// Background goroutine retries periodically.
r.Replay(func(data []byte) error {
    _, err := httpClient.Post(serviceB, data)
    return err
})
r.Purge()
```

---

## 4. Edge / IoT Data Collection

Devices deployed at the edge often have intermittent or unreliable network connectivity. Sensor readings, telemetry, or logs must not be lost during offline periods.

**With rebuf:** store readings on the local filesystem and flush them to the server when connectivity returns.

```go
reading := sensor.Read()
r.Write(reading)

// When network is available, upload buffered readings.
if network.IsAvailable() {
    r.Replay(func(data []byte) error {
        return api.Upload(data)
    })
    r.Purge()
}
```

---

## 5. Log Shipping and Forwarding

A log forwarder collects application logs and ships them to a remote aggregator (e.g., Elasticsearch, Loki, Splunk). If the aggregator is temporarily unreachable, logs pile up in memory or get dropped.

**With rebuf:** persist logs to a segmented WAL on disk and forward them when the aggregator is back online.

```go
func forward(entry []byte) {
    if err := aggregator.Send(entry); err != nil {
        r.Write(entry)
    }
}

// Periodic drain.
r.Replay(func(data []byte) error {
    return aggregator.Send(data)
})
r.PurgeThrough(lastSuccessfulSegment)
```

---

## 6. Payment and Transaction Durability

Financial transactions demand at-least-once delivery. A payment gateway timeout should never result in a silently lost charge or refund.

**With rebuf:** persist the transaction payload before attempting the call. On failure, replay ensures every transaction is retried.

```go
r.Write(serialize(transaction))

if err := gateway.Process(transaction); err != nil {
    // Will be retried on next replay cycle.
    return
}

r.Purge()
```

---

## 7. Webhook Delivery Guarantees

Your platform dispatches webhooks to external endpoints. Those endpoints may be temporarily down, rate-limited, or slow to respond.

**With rebuf:** write webhook payloads to the WAL. A background worker replays and delivers them with retries, then purges successfully delivered segments.

```go
// On event trigger.
r.Write(webhookPayload)

// Background delivery worker.
r.Replay(func(data []byte) error {
    resp, err := http.Post(endpoint, "application/json", bytes.NewReader(data))
    if err != nil || resp.StatusCode >= 500 {
        return fmt.Errorf("delivery failed")
    }
    return nil
})
r.Purge()
```

---

## 8. Batch Processing Pipeline

A data pipeline ingests records one at a time but processes them in batches for efficiency. If the pipeline crashes mid-batch, in-flight records are lost.

**With rebuf:** write each incoming record to the WAL. The batch processor replays a window of records, processes them together, then purges the consumed segments.

```go
// Ingest phase.
for record := range incomingStream {
    r.WriteBatch(recordBatch)
}

// Batch process phase.
var batch []Record
r.Replay(func(data []byte) error {
    batch = append(batch, deserialize(data))
    return nil
})
processBatch(batch)
r.Purge()
```

---

## Why rebuf?

| Concern | How rebuf helps |
|---|---|
| **Durability** | Data is written to disk with CRC32 checksums — survives process crashes. |
| **Simplicity** | Zero external dependencies. No broker, no database — just files. |
| **Concurrency** | Mutex-protected operations are safe for use across goroutines. |
| **Backpressure** | Configurable segment size and retention prevent unbounded disk growth. |
| **Selective replay** | Replay from a specific segment to avoid reprocessing old data. |
| **Lightweight** | Embeds directly into your Go binary with no operational overhead. |

---

For API details and configuration options, see the main [README](README.md).
