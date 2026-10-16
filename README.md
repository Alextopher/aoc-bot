# aoc-bot

![image](https://github.com/Alextopher/aocbot/assets/56850185/7241d3f5-a092-47fd-91e9-4b11a4017426)

A Discord bot that follows an Advent of Code private leaderboard. It hands out
roles for star milestones and unlocks per-day discussion channels once a member
has finished that day's puzzle.

This is a personal hobby project built for one server. It's not set up for other
users, and there are no plans to support them.

## Running it

Write a `config.json` next to the binary:

```json
{
  "discord_token": "...",
  "session_cookie": "...",
  "guilds": {
    "<guild id>": {
      "year": "2025",
      "leaderboard_id": "<leaderboard id>",
      "daily_roles": true
    }
  }
}
```

`session_cookie` is the value of the `session` cookie from a logged-in
adventofcode.com browser session.

Then:

```sh
docker compose up -d --build
```

Or run it directly with `go run .`. Either way the process needs a writable
`logs/` directory, where it keeps one append-only claim log per guild.

The bot polls the leaderboard API every 15 minutes and reconciles roles from
what it sees.

## Roles

Created on startup, removed when they no longer apply:

- `Connected` — the member has claimed an Advent of Code account.
- `Spoiler` — opt-in access to every day's channel, via `/spoilers`.
- `N Stars` — hoisted milestone roles at 4, 8, 12, 16, 20, and 24 stars.
- `Day NN` — access to one day's channel, granted on completion
  (`daily_roles` guilds only).

The event is currently 12 days long; `numDays` and `starTiers` in `bot.go`
define the shape, and stale roles from a previous shape are deleted at startup.

## Commands

| Command | Description |
| --- | --- |
| `/claim <username>` | Link your Discord account to an Advent of Code name or ID |
| `/unclaim [member]` | Drop your claim, or someone else's (admin) |
| `/stars [member]` | Show a star count |
| `/spoilers` | Toggle access to all day channels |
| `/setup <day>` | Mark the current channel as that day's channel (admin) |
| `/source` | Link to this repository |
| `/help` | List commands |

## License (MIT)

Copyright (c) 2023-2026 Christopher Mahoney

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
