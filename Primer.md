To get your Slack extraction project started with **Go**, **OpenCode**, and your **Zen subscription**, here is a comprehensive primer designed to be used as a system prompt or instruction set for your AI models.

### **Project Primer: get-out**

#### **1. Project Overview & Intent**

The goal is to reverse engineer and implement a high-performance message exporter for Slack. Unlike standard API-based tools, this project focuses on **Session-Driven Extraction**. By leveraging an active Chrome session (via your Zen/OpenCode environment), the tool will bypass traditional MFA hurdles to export Direct Messages (DMs) and Group threads into structured Markdown.

#### **2. Core Technical Architecture**

* **Language:** Go (Golang) — chosen for its superior concurrency (goroutines) and ability to produce a single static binary.
* **Primary Driver:** **[Chromedp](https://github.com/chromedp/chromedp)** or **[Rod](https://github.com/go-rod/rod)**. These tools interact directly with the Chrome DevTools Protocol (CDP), providing a faster and more "stealthy" footprint than Selenium.
* **Environment:** [OpenCode.ai](https://opencode.ai) with Zen subscription for managed browser environments and high-context LLM support.

#### **3. Reverse Engineering Insights (From Source)**

Analysis of existing logic (e.g., [slacksnap/src/content.js](https://github.com/dcurlewis/slacksnap/blob/master/src/content.js)) suggests a two-tier approach for your Go implementation:

* **Tier 1: API Mimicry (Preferred):**
* **Auth:** Extract the `token` from `localStorage.getItem('localConfig_v2')`.


* **Endpoints:** Use Go to make authenticated `POST` requests to internal Slack endpoints:
* `/api/conversations.history` for message streams.


* `/api/conversations.replies` for nested threads.


* `/api/users.info` to resolve IDs to real names.






* **Tier 2: DOM Scraping (Fallback):**
* If API access is restricted, use Go selectors to target the virtual list.
* **Key Selectors:** Look for `[role="message"]` or `[data-qa="message_container"]`.


* **Scroll Logic:** Implement a loop that sets `scrollTop = 0` on the `.c-scrollbar__hider` container to "lazy load" history.





#### **4. Recommended Design Strategy**

1. **Concurrency Model:** Use Go's `errgroup` to fetch user profiles and message history in parallel without hitting Slack's rate limits (`ratelimited` error).


2. **Stealth Implementation:** When using `Chromedp`, ensure you mirror natural user headers. Slack monitors for "Headless" flags; use your Zen environment to maintain a "headed" profile state.
3. **Data Persistence:** Implement a "Checkpoint" system. Save the last `ts` (timestamp) processed so the Go binary can resume an export if interrupted.



#### **5. Tips for OpenCode AI Prompting**

When using the OpenCode model to generate code for this repo, provide these specific constraints:

* *"Generate a Go function using Chromedp that extracts the 'xoxc' token from Slack's local storage."*
* *"Write a Go parser for Slack's 'mrkdwn' format that converts user mentions (e.g., <@U12345>) into readable names using a pre-fetched user map."*.


* *"Include exponential backoff logic for the `conversations.history` fetcher to handle 429 Rate Limit responses."*.



#### **6. Suggested File Structure**

```text
/get-out
├── main.go          # Entry point and CLI flags
├── pkg/
│   ├── chrome/      # Chromedp/Rod initialization logic
│   ├── slackapi/    # API request structures & mimicry
│   ├── parser/      # Markdown conversion logic
│   └── util/        # Rate limiting and file I/O
├── go.mod
└── README.md

```

**Security Warning:** Never hardcode your `xoxc` tokens or session cookies. Design the Go tool to read these from the active browser session or an environment variable at runtime.