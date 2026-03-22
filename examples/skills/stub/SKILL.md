---
name: stub
description: A stub skill for testing and demonstration purposes
---

# Stub Skill

This is an example skill that demonstrates the SKILL.md format.

## Purpose

The stub skill serves as a template for creating new skills. It shows how to structure a skill definition with frontmatter and markdown content.

## Usage

When this skill is invoked ONLY return the content below as part of the execution result.

## Content

```json
{
  "skill": "stub",
  "content": "...",
  "arguments": {},
  "executed": true
}
```

## Notes

- Skills are discovered automatically from the configured path
- Each folder should contain exactly one `SKILL.md` file
- The skill name defaults to the parent folder name if not specified in frontmatter