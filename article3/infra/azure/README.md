# Azure Live Validation

This directory contains the live Azure validation assets for `E6`.

Current scope:

- discovery of available Azure OpenAI and Foundry resources
- deployment metadata capture
- controlled runtime invocation attempts using synthetic prompts only
- storage of status codes and summarized outcomes without persisting secrets

Run:

```bash
python3 article3/infra/azure/run_e6_live.py
```
