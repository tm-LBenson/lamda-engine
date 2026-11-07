Cooldown baselines extracted from SimulationCraft commit `fa22f84d2a13255d7325826f89a46bbb9181233d`, SpellDataDump/allspells.txt, WoW 12.1.0.69933 Live. Source: https://github.com/simulationcraft/simc/tree/fa22f84d2a13255d7325826f89a46bbb9181233d

Use recharge duration when present. These are baseline estimates, not opponent talent inspection. Charges are explicitly uncertain. No aura-expiration-to-cast inference. Racial variants retain their actual observed spell IDs; shared racial/trinket locks are owned by the native trinket widget.

School-lockout baselines come from the interrupt spell duration (Skull Bash uses effect 93985). The engine requires a successful interrupt record and a verified destination GUID; it does not turn an attempted interrupt into a lockout. PvP/talent modifiers can change duration, so the display is explicitly approximate.
