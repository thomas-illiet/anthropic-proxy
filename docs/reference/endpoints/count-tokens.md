# Count Tokens Endpoint

`POST /v1/messages/count_tokens` returns a local token count for an Anthropic-compatible request.

```bash
curl http://127.0.0.1:8787/v1/messages/count_tokens \
  -H "content-type: application/json" \
  -d '{"model":"claude-test","messages":[{"role":"user","content":"hello"}]}'
```

Example response:

```json
{
  "input_tokens": 12
}
```

The value is calculated with a local tiktoken-compatible tokenizer after converting the Anthropic request into the OpenAI-compatible request shape that will be sent upstream. Unknown model tokenizers fall back to `cl100k_base`.
