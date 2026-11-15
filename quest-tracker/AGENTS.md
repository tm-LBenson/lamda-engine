# Lamda Quest Log

Standalone native WoW quest tracker. Forever is the primary target; Retail is
secondary. Canonical source is this directory. Do not edit installed copies.

- Read README.md and LamdaQuestLog.toc before changes.
- No dependencies on LamdaUI, companion services, or external UI libraries.
- Preserve client and character separation through SavedVariablesPerCharacter.
- Defer protected frame changes until combat ends. Never execute stored Lua.
- Do not install, change live settings/macros/bindings, or publish without Lewis's
  applicable authorization. Do not stop or restart WoW.
- Keep temporary tooling under .tmp; package only the explicit runtime files.
- Run tools/check.py with a Python environment containing lupa (Lua 5.1).
- In-game behavior and visual parity require a real client check; mocks cannot
  establish either. Record those limits honestly.
