# Structured player test protocol

Do not ask only “did you like it?”. Each session combines observed behaviour, telemetry, and a very short post-play rating.

## Session format
1. Player starts without verbal coaching unless the test specifically measures teaching.
2. Record mode, build, prior experience and whether the tutorial was used.
3. Let the player play naturally for at least 15 minutes or until they voluntarily stop.
4. For each failure/confusion point record *what happened before giving an explanation*.
5. After each mode, ask four 1–7 ratings: enjoyment, fairness, clarity, “I want another run”.
6. Ask one open question: “What was the best moment?” and one: “What was annoying or unclear?”

## Automatically compare
- tutorial completion
- first-success time
- run count before voluntary stop
- failure category distribution
- retry-after-failure rate
- boss attempt → clear conversion
- session duration
- mode switching
- profile/achievement/cache discovery
- frame-time/hitch counters on the same sessions

## Decision rules
- High failure + high retry + high fairness = likely healthy challenge.
- High failure + low fairness/clarity = telegraph/readability issue before difficulty issue.
- Low failure + low “another run” = likely boredom/underchallenge.
- A mechanic with strong completion but poor enjoyment is not automatically successful.
- Never nerf/buff from fewer than ~20 comparable observations unless there is a clear defect.
- Prefer A/B-style changes one variable at a time when practical.
