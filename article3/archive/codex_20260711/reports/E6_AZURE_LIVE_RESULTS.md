# E6 Azure Live Results

Live discovery and invocation attempts against Azure resources already present in the subscription.

```json
{
  "experiment_id": "E6",
  "variant": "azure_live_discovery_and_invocation",
  "subscription": {
    "id": "60f9b06a-66b1-437f-ac92-3b5e9720e34a",
    "name": "FHIR",
    "tenant_id": "f75c19a9-c006-47d7-98cc-f1b5638e6af6"
  },
  "resource_count": 4,
  "resources": [
    {
      "name": "greenops-fdry-60f9b0",
      "resource_group": "rg-greenops-fresh-20260708",
      "location": "westus2",
      "kind": "AIServices"
    },
    {
      "name": "a2fdry60f9b0hkw0yk",
      "resource_group": "rg-article2-governance-20260708",
      "location": "westus2",
      "kind": "AIServices"
    },
    {
      "name": "a2fr60f9b020260708",
      "resource_group": "rg-article2-governance-20260708",
      "location": "francecentral",
      "kind": "OpenAI"
    },
    {
      "name": "a2us60f9b020260708",
      "resource_group": "rg-article2-governance-20260708",
      "location": "eastus",
      "kind": "OpenAI"
    }
  ],
  "deployments": {
    "gpt_france_mini": {
      "account": "a2fr60f9b020260708",
      "location": "rg-article2-governance-20260708",
      "model": {
        "callRateLimit": null,
        "format": "OpenAI",
        "name": "gpt-4.1-mini",
        "source": null,
        "version": "2025-04-14"
      },
      "capabilities": {
        "agentsV2": "true",
        "area": "EUR",
        "assistants": "true",
        "chatCompletion": "true",
        "responses": "true"
      }
    },
    "gpt_us_mini": {
      "account": "a2us60f9b020260708",
      "location": "rg-article2-governance-20260708",
      "model": {
        "callRateLimit": null,
        "format": "OpenAI",
        "name": "gpt-4.1-mini",
        "source": null,
        "version": "2025-04-14"
      },
      "capabilities": {
        "agentsV2": "true",
        "area": "US",
        "assistants": "true",
        "chatCompletion": "true",
        "responses": "true"
      }
    },
    "mistral_large_latest": {
      "account": "greenops-fdry-60f9b0",
      "location": "rg-greenops-fresh-20260708",
      "model": {
        "callRateLimit": null,
        "format": "Mistral AI",
        "name": "Mistral-Large-3",
        "source": null,
        "version": "1"
      },
      "capabilities": {
        "chatCompletion": "true"
      }
    }
  },
  "attempts": {
    "france_deployment_chat_2024_10_21": {
      "status_code": "200",
      "curl_exit_code": 0,
      "content": "OK",
      "usage": {
        "completion_tokens": 2,
        "completion_tokens_details": {
          "accepted_prediction_tokens": 0,
          "audio_tokens": 0,
          "reasoning_tokens": 0,
          "rejected_prediction_tokens": 0
        },
        "latency_checkpoint": {
          "engine_tbt_ms": 4,
          "engine_ttft_ms": 25,
          "engine_ttlt_ms": 34,
          "pre_inference_ms": 85,
          "service_tbt_ms": 6,
          "service_ttft_ms": 133,
          "service_ttlt_ms": 141,
          "total_duration_ms": 59,
          "user_visible_ttft_ms": 48
        },
        "prompt_tokens": 11,
        "prompt_tokens_details": {
          "audio_tokens": 0,
          "cached_tokens": 0
        },
        "total_tokens": 13
      }
    },
    "us_deployment_chat_2024_10_21": {
      "status_code": "200",
      "curl_exit_code": 0,
      "content": "OK",
      "usage": {
        "completion_tokens": 2,
        "completion_tokens_details": {
          "accepted_prediction_tokens": 0,
          "audio_tokens": 0,
          "reasoning_tokens": 0,
          "rejected_prediction_tokens": 0
        },
        "latency_checkpoint": {
          "engine_tbt_ms": 4,
          "engine_ttft_ms": 30,
          "engine_ttlt_ms": 39,
          "pre_inference_ms": 312,
          "service_tbt_ms": 76,
          "service_ttft_ms": 481,
          "service_ttlt_ms": 585,
          "total_duration_ms": 310,
          "user_visible_ttft_ms": 169
        },
        "prompt_tokens": 11,
        "prompt_tokens_details": {
          "audio_tokens": 0,
          "cached_tokens": 0
        },
        "total_tokens": 13
      }
    },
    "france_v1_responses": {
      "status_code": "200",
      "curl_exit_code": 0
    },
    "us_v1_responses": {
      "status_code": "200",
      "curl_exit_code": 0
    },
    "mistral_services_openai_v1_chat": {
      "status_code": "200",
      "curl_exit_code": 0,
      "content": "OK",
      "usage": {
        "prompt_tokens": 7,
        "completion_tokens": 2,
        "total_tokens": 9,
        "audio_prompt_tokens": 0
      }
    },
    "mistral_services_models_chat_preview": {
      "status_code": "200",
      "curl_exit_code": 0,
      "content": "OK",
      "usage": {
        "prompt_tokens": 7,
        "completion_tokens": 2,
        "total_tokens": 9,
        "audio_prompt_tokens": 0
      }
    }
  }
}
```
