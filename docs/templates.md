# Contributing Templates to the Community Library

The symvibe community template library lets anyone share reusable Baukasten
cycles as templates that any symvibe user can discover and import directly from
the board's **Library** panel.

## How it works

The library is a Git-backed index — no hosted search backend is needed. The
board fetches a single `index.json` file from a public GitHub repository (raw
content URL) and renders searchable cards. Clicking **Template verwenden**
downloads the referenced template JSON and opens the standard import dialog with
capability checking and optional category remapping.

## index.json schema

The index file is a JSON array. Each entry must conform to:

```json
[
  {
    "id": "go-tdd",
    "name": "Go TDD Cycle",
    "author": "alice",
    "tags": ["go", "tdd", "testing"],
    "description": "A test-driven Go development cycle with lint, test, and release phases.",
    "url": "https://raw.githubusercontent.com/danieljustus/symvibe-templates/main/templates/go-tdd.json"
  }
]
```

| Field | Type | Required | Notes |
|---|---|---|---|
| `id` | string | ✅ | Unique slug, lowercase + hyphens |
| `name` | string | ✅ | Human-readable template title |
| `author` | string | ✅ | GitHub username or full name |
| `tags` | string[] | — | Used for filtering; prefer lowercase |
| `description` | string | — | One-sentence summary |
| `url` | string | ✅ | Direct raw URL to the template JSON |

## Template JSON format

Each template must be a valid symvibe template — the same format exported via
the board's **Export** button:

```json
{
  "kind": "symvibe.template",
  "schema_version": 1,
  "manifest": {
    "id": "go-tdd",
    "name": "Go TDD Cycle",
    "version": "1.0.0",
    "author": "alice",
    "tags": ["go", "tdd"],
    "description": "A test-driven Go development cycle."
  },
  "requires": {
    "skills": ["commit"],
    "categories": ["deep", "quick"],
    "agents": [],
    "sensors": []
  },
  "phases": [
    {
      "id": "plan",
      "name": "Plan",
      "steps": [
        {
          "id": "spec",
          "name": "Write specification",
          "category": "deep",
          "skill": "",
          "prompt": "Write a clear specification for the feature."
        }
      ]
    }
  ]
}
```

The `requires` block is checked by symvibe before importing — users without the
listed skills, categories, agents, or sensors are offered a remapping dialog.

## How to contribute

1. **Fork** the templates repository:
   `https://github.com/danieljustus/symvibe-templates`

2. **Export your cycle** from the symvibe board → **Export** → saves template
   JSON to clipboard.

3. **Add your template file** under `templates/<your-id>.json`.

4. **Register your entry** in `index.json` at the repo root, following the
   schema above.

5. **Open a pull request** against `main`. Include:
   - A brief description of what the cycle does.
   - The categories / skills / agents it requires.
   - A short demo or screenshot if applicable.

## Changing the library source

By default symvibe fetches the community index from:
```
https://raw.githubusercontent.com/danieljustus/symvibe-templates/main/index.json
```

To point at your own fork or a private index, set `library_index_url` in
`~/.config/symvibe/config.toml`:

```toml
[server]
library_index_url = "https://raw.githubusercontent.com/your-org/your-templates/main/index.json"
```

The board caches the index for 5 minutes per session; restart symvibe to force
an immediate refresh.
