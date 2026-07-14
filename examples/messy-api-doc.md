# Payments API (example document)

This file is deliberately messy: it collects the curl shapes that rot in
real READMEs. Run curlfmt over it to see every one rewritten while the
prose, the Python block, and the JSON block stay byte-identical.

Create a payment (one gnarly line, short flags, unquoted JSON-ish body):

```bash
curl -sSLX POST http://127.0.0.1:8080/v1/payments -H 'content-type:application/json' -H 'accept: application/json' -u admin:hunter2 -d '{"amount":1999,"currency":"EUR"}' -o resp.json
```

Fetch one, console style with a prompt:

```console
$ curl -s -X GET http://127.0.0.1:8080/v1/payments/42
{"id": 42, "amount": 1999}
```

Poll until settled, piping into jq:

```sh
curl -s http://127.0.0.1:8080/v1/payments/42 | jq '.status'
```

A multi-line body already using continuations (still gets canonical
option order and long flags):

```bash
curl -X POST 'http://127.0.0.1:8080/v1/refunds' \
  -H 'content-type: application/json' \
  -d '{
  "payment_id": 42,
  "reason": "duplicate"
}'
```

Not shell — curlfmt must never touch these:

```python
curl = "just a variable name"
```

```json
{"curl": "-sSL is data here"}
```
