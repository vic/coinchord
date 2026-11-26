# CoinChord: Type at CharaChorder Speeds Using Everyday Desk Objects & Your Laptop Mic

[![Sponsor](https://img.shields.io/badge/sponsor-vic-ea4aaa?logo=githubsponsors)](https://github.com/sponsors/vic)

![genesis](./genesis.gif)

Welcome to **CoinChord**—an acoustic word-chording keyboard that requires **zero new hardware, zero cameras, and zero batteries**. All you need is your laptop's built-in microphone and two small objects sitting on your desk.

### How It Works (In Plain English)
Normal keyboards force you to hunt down one letter at a time. Radical chording keyboards like the **CharaChorder** let you hit 250+ WPM by pressing combinations that output entire words at once.

CoinChord brings that exact same whole-word chording idea to your desk using **sound physics**:
1. Your laptop microphone listens at **44,100 samples per second**—over 700x faster than a standard webcam.
2. When you tap or slide two different objects on your desk, each object produces a distinct **metallic pitch** (frequency) and **friction signature** (sharp ring vs. rough scrape).
3. CoinChord listens to those micro-sounds, groups rapid two-beat rhythms into chords, and turns them directly into full English words or programming constructs.

---

## 1. The Essentials: What Objects Should You Choose?

You do **not** need special gear, and you don't even need coins if you don't carry cash. CoinChord only needs **two desk strikers that sound noticeably different from each other**:
* **Left Hand = High Pitch (~8,500 – 12,000 Hz):** Something small, light, or sharp that makes a bright *"tink"*.
* **Right Hand = Low Pitch (~4,500 – 7,500 Hz):** Something larger, heavier, or denser that makes a deeper *"clink"* or *"thunk"*.

### Best Everyday Pairs (Ranked by Accuracy)

| Rank | Left Hand (High Pitch) | Right Hand (Low Pitch) | Feel & Acoustic Performance |
| :--- | :--- | :--- | :--- |
| **1** | **Small Coin** (e.g., \$1 MXN, Dime, €0.10) | **Large Coin** (e.g., \$10 MXN, Quarter, €2) | **The Gold Standard.** Cleanest metallic ring, effortless to slide across a desk, and fits naturally in your fingers. |
| **2** | **House Key** (brass or steel) | **Metal Spoon** or **Butter Knife** | **Maximum Contrast.** Keys ring super bright; spoons produce a deep, unmistakable bell tone. |
| **3** | **Plastic Pen Cap** | **Heavy Metal Pen** or **Lighter** | **Desk Native.** Great if you already fidget with pens while thinking or coding. |
| **4** | **Guitar Pick** | **Coffee Mug Rim** or **Wood Coaster** | **Speed Demon.** A pick double-taps effortlessly on the left while the right hand anchors a solid beat. |
| **5** | **Your Bare Fingernail** | **Wedding Ring** or **Knuckle** | **True Zero-Object.** Flicking a fingernail clicks high; tapping a ring or knuckle thuds low. |

### What About Your Desk Surface?
* **Best surfaces:** Solid wood, laminate, glass, stone, or a thick hardcover book. These reflect high-frequency sound cleanly straight into your laptop mic.
* **Avoid soft cloth mats:** A thick foam/cloth gaming mousepad absorbs the metallic ring. If you use one, just slide a hardcover notebook, wooden coaster, or clipboard under your wrists.

## 2. How to Hold Your Strikers (Zero Arm Fatigue)

The biggest mistake beginners make is hovering their hands in the air like they are playing an invisible piano. That causes shoulder fatigue in two minutes. Instead, use the **Desk-Anchored Pinch**:

1. **Rest the heels of both palms flat on the desk** right in front of your laptop, just like you're resting at a normal keyboard. Your wrists stay planted the entire time.
2. **Pinch each object lightly between your thumb and index finger**—just like holding a guitar pick—with about **half of the object sticking out** below your fingertips.
3. **Hold near the edge, never the dead center.** Squeezing the middle of a coin or spoon chokes its vibration (like grabbing a ringing bell). Holding near the rim lets the metal sing clearly.
4. **Keep your movements tiny.** A **3 to 5 millimeter flick** of your thumb or wrist is plenty. If your hand moves more than half an inch, you're working too hard.

---

## 3. Quick Start & Calibration

If you are using Nix, launch the interactive suite directly from the project folder:

    nix run

Or enter the development environment:

    nix develop
    go run main.go

When the main menu appears, always start with **Option 1 (`Setup & Calibrate`)**:
1. **Stay quiet for 2 seconds** while CoinChord measures your room's baseline noise floor (AC, laptop fan, street hum).
2. **Tap your Left object 5 times** at a relaxed, natural volume.
3. **Tap your Right object 5 times.**
4. **Slide/scrape either object ~1 inch across the desk 5 times** (like striking a match).

> **Check your Pitch Gap:** At the end of calibration, CoinChord prints your `Gap` in Hz. Anything above **1,500 Hz** gives rock-solid left/right separation. If your gap is under 1,000 Hz, swap one of your objects for something noticeably larger or smaller and re-calibrate.

## 4. Learning Path: From First Tap to 250+ WPM

Think of CoinChord like learning three chords on a guitar—once your hands feel the rhythm, muscle memory takes over completely. Practice in four short stages:

### Stage 1: The Three Core Primitives (Day 1 — 5 Minutes)
Every word in CoinChord is built from just **three physical sounds**:
* **`L` (Left Tap):** One crisp tap with your left hand -> outputs **`the `**
* **`R` (Right Tap):** One crisp tap with your right hand -> outputs **`and `**
* **`S` (Desk Scrape):** Drag the edge of either coin ~1 inch across the table -> outputs **`in `**

**Warmup Drill:** Tap *Left -> Right -> Scrape -> Left* slowly:
`the and in the `

### Stage 2: Rhythm & Simultaneous Hits (Day 2 — 5 Minutes)
Once single taps feel natural, add bounce and two-hand drops:
* **`LL` (Left Double-Tap):** Two quick bounces within 220ms (like dribbling a basketball) -> outputs **`to `**
* **`RR` (Right Double-Tap):** Two quick bounces with your right hand -> outputs **`of `**
* **`D` (Dual Simultaneous):** Drop both objects onto the desk at the exact same instant -> outputs **`I `**

**Warmup Drill:** Try *Dual -> Left Double -> Right Double*:
`I to of `

### Stage 3: Two-Beat Word Chords for English Prose (Days 3–5)
This is where CharaChorder-level speed happens. Any two gestures performed within **330 milliseconds** merge into a single whole-word chord. Don't think of them as two separate taps—think of them as a single two-syllable drumbeat (*"ta-dum"*):

| Chord | How It Feels Physically | Output | Memory Trick |
| :--- | :--- | :--- | :--- |
| `L+R` | Left-Right gallop (*"ta-dum"*) | `that ` | Forward roll left-to-right |
| `R+L` | Right-Left gallop (*"dum-ta"*) | `you ` | Pointing back at "you" |
| `L+S` | Tap Left, immediately slide | `with ` | Left tap + sliding "with" you |
| `R+S` | Tap Right, immediately slide | `for ` | Right tap + reaching "for"ward |
| `LL+R` | Left double-bounce + Right tap | `is ` | Quick stutter then lands |
| `RR+L` | Right double-bounce + Left tap | `it ` | Mirror of `is` |
| `S+S` | Quick back-and-forth scrub | `ing ` | Continuous action suffix |
| `D+S` | Both tap together, then slide | `.\n` | Full stop + sweep to next line |

**The Fluency Sentence:** Practice this 6-chord sequence until it flows like a single musical phrase:
1. `R+L` -> `you `
2. `L+R` -> `that `
3. `LL+R` -> `is `
4. `R+S` -> `for `
5. `L` -> `the `
6. `D+S` -> `.\n`

> Result: **`you that is for the .`** in under 1.5 seconds!

Run **Option 2 (`Interactive Trainer`)** in the app to practice all 9 core lessons and test your real-time WPM on this exact drill.

## 5. Programming Mode

Typing code on a normal keyboard punishes your pinkies with awkward reaches for `=`, `()`, `:`, and indentation. CoinChord includes a dedicated **Programming Mode** where common Python/C-style syntax becomes effortless one-beat gestures.

### Switching Modes Mid-Sentence
Perform **`LL+RR`** (Left double-tap followed immediately by Right double-tap—*"ta-ta dum-dum"*) anytime to toggle between **Prose Mode** and **Code Mode**.

| Chord | Code Output | Why It's Mapped Here |
| :--- | :--- | :--- |
| `L` | `def ` | Left hand starts a function definition |
| `R` | `return ` | Right hand finishes and returns a value |
| `LL` | `if ` | Quick double-check on the left |
| `RR` | `else:\n    ` | Fallback on the right (with auto-indent!) |
| `S` | ` = ` | Scraping a horizontal line draws an equals sign |
| `D` | `()` | Two objects hitting together = two parentheses |
| `L+R` | `for ` | Left-to-right loop progression |
| `R+L` | ` in ` | Stepping inside a collection |
| `L+S` | `import ` | Pulling a library onto the desk |
| `R+S` | `print(` | Sliding output out to the terminal |
| `LL+R` | `True` | Positive boolean |
| `RR+L` | `False` | Negative boolean |
| `S+S` | `:\n    ` | Double-scrape drops a colon + newline + 4 spaces |
| `D+S` | `\n` | Clean newline |

### Writing a Complete Function in 7 Gestures
1. `L` -> `def `
2. *(type function name)* + `D` -> `()`
3. `S+S` -> `:\n    `
4. `L+R` -> `for `
5. *(var)* + `R+L` -> ` in `
6. *(list)* + `S+S` -> `:\n    `
7. `R` -> `return `

---

## 6. Advanced Topics: Pushing Speed & Accuracy

### 1. Eliminating False Triggers in Noisy Rooms
CoinChord ignores low-frequency sounds below **1,800 Hz** (so human voices, footsteps, and bass hum won't trigger keystrokes). However, mechanical keyboards or clinking dishes can occasionally cross the threshold.
* Open `~/.coinchord.json` and raise `"trigger"` slightly (e.g., from `0.032` to `0.050`).
* Position your hands slightly closer to your laptop's microphone pinhole (usually near the top bezel or palm rest edge) so you can tap softer while maintaining a high signal-to-noise ratio.

### 2. Tuning Your Chord Window (`330 ms`)
* **Beginners:** If your two-beat chords (`L+R`) are splitting into separate words (`the and ` instead of `that `), increase the `330 * time.Millisecond` window in `main.go` to `420 * time.Millisecond`.
* **Speed Typists (200+ WPM):** Once your hands get fast, tighten the window down to `220 * time.Millisecond` and the double-tap threshold (`220 ms`) down to `160 ms` so rapid single words don't accidentally merge.

### 3. Adding Your Own Custom Chords
Open `main.go` and add entries to `proseMap` or `codeMap` using any combination of `L`, `R`, `LL`, `RR`, `S`, and `D`:
* Single-beat keys: `"L"`, `"R"`, `"LL"`, `"RR"`, `"S"`, `"D"`
* Two-beat combos: `"L+R"`, `"R+L"`, `"L+S"`, `"R+S"`, `"S+L"`, `"S+R"`, `"LL+R"`, `"RR+L"`, `"D+L"`, `"D+R"`, `"D+S"`

---

## 7. `coinchord` Complete Reference

### CLI & Interactive Menu
* **`nix run`** or **`go run main.go`** — Launches the interactive CoinChord console suite:
  * **`1) Setup & Calibrate`** — Measures 2s of ambient noise, captures 5 left taps, 5 right taps, and 5 scrapes, and saves thresholds to `~/.coinchord.json`.
  * **`2) Interactive Trainer`** — Runs the 9-lesson guided accuracy check followed by a timed 5-chord WPM speed test.
  * **`3) Live Prose Mode`** — Starts real-time English word-chording output.
  * **`4) Live Code Mode`** — Starts real-time programming syntax chording output.

### Configuration File (`~/.coinchord.json`)

| Field | Default | Description |
| :--- | :--- | :--- |
| `pair` | `"Small Coin (L) + Large Coin (R)"` | Label for your calibrated striker pair. |
| `trigger` | `0.032` | Minimum RMS volume (1.8–18.5 kHz band) required to register a strike. |
| `left_hz` | `9200` | Median spectral centroid (Hz) of your Left-hand striker. |
| `right_hz` | `5800` | Median spectral centroid (Hz) of your Right-hand striker. |
| `scrape_flt` | `0.28` | Spectral flatness cutoff separating noisy desk scrapes (`> 0.28`) from pure metallic pings (`< 0.28`). |

### Audio Engine Specifications
* **Sample Rate:** `44,100 Hz` mono, 16-bit signed PCM (`S16_LE`) via `arecord` (Linux) or `rec` / SoX (macOS).
* **Frame Size:** `1,024 samples` (~23.2 ms latency per analysis window) with Hann windowing and Radix-2 FFT.
* **Classification Logic:**
  1. **Scrape (`S`):** Burst duration `> 115 ms` OR spectral flatness `> scrape_flt`.
  2. **Dual (`D`):** Multi-frame burst whose spectral centroid lands within 16% of the midpoint between `left_hz` and `right_hz`.
  3. **Left (`L`) vs. Right (`R`):** Nearest neighbor match to `left_hz` vs. `right_hz`.

