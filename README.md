# CoinChord: Type at CharaChorder Speeds Using Everyday Desk Objects & Your Laptop Mic

[![Sponsor](https://img.shields.io/badge/sponsor-vic-ea4aaa?logo=githubsponsors)](https://github.com/sponsors/vic)

![genesis](./genesis.gif)
# CoinChord: Type at CharaChorder Speeds with Two Desk Objects & Your Laptop Mic

Welcome to **CoinChord**—an acoustic word-chording keyboard that turns your laptop's built-in microphone and two everyday objects on your desk into a high-speed text input engine. No custom hardware, no webcams, no Bluetooth dongles, and zero batteries.

### The Core Mindset: Stop Spelling, Start Conducting
Most keyboards trap you in a 19th-century bottleneck: hunting down one letter at a time (`t` -> `h` -> `e` -> `space`). Radical chording keyboards like the **CharaChorder** break the 250+ WPM barrier by letting you trigger **entire words and phrases as a single physical gesture**.

CoinChord brings that exact philosophy to your existing laptop using **acoustic geometry**:
* **Think like a drummer or pianist, not a typist.** You aren't pressing tiny buttons; you are performing crisp **taps**, straight **lines**, sweeping **curves**, and circular **swirls** on your desk.
* **Your microphone is a 44,100 Hz radar.** While a webcam struggles at 30 frames per second in dim light, your laptop's mic captures 44,100 samples every second—easily separating your left hand from your right hand by resonant pitch, and telling a straight drag apart from a curved hook or a circular swirl.
* **Bind concepts to motion.** Never mentally spell out `"w-i-t-h"`. Instead, teach your hands that *"Left draws a line while Right hooks a curve"* simply **means** `"with "`. Once your motor cortex links the shape directly to the word, your hands flow at conversational speech speed.

---

## 1. What Objects Should You Use? (Coins & Everyday Alternatives)

CoinChord doesn't care what is in your hands—it only needs **two strikers that make noticeably different pitches** when they touch or slide across your desk:
* **Left Hand = High Pitch (~8.5–12 kHz):** Something small, light, or sharp that rings with a bright *"tink"*.
* **Right Hand = Low Pitch (~4.5–7.5 kHz):** Something larger, heavier, or denser that rings with a deeper *"clink"* or *"thunk"*.

### The Best Desk Pairs You Already Own

| Pair | Left Hand (High Pitch) | Right Hand (Low Pitch) | What It Feels Like |
| :--- | :--- | :--- | :--- |
| **Pocket Coins** | Small coin (e.g., \$1 MXN, Dime, €0.10) | Large coin (e.g., \$10 MXN, Quarter, €2) | **The Gold Standard.** Smooth edges glide effortlessly into circles and curves while ringing clearly. |
| **Key & Spoon** | House key (brass or steel) | Metal spoon or butter knife | **Maximum Contrast.** Almost impossible for the mic to confuse, even in a noisy café. |
| **Desk Pens** | Light plastic pen cap | Heavy metal pen or lighter | **Office Native.** Natural if you already fidget with pens while thinking or coding. |
| **Zero-Object** | Your bare fingernail | Wedding ring or knuckle | **Always With You.** Flick a fingernail for a sharp high click; tap a ring or knuckle for a low thud. |
| **Musician's Setup** | Guitar pick | Ceramic mug rim or wooden coaster | **Ultra-Fast.** Effortless high-speed taps and sweeps with zero hand tension. |

> **A Quick Note on Desk Surfaces:** Hard wood, laminate, glass, stone, or a thick hardcover book reflect crisp high-frequency sound straight into your laptop mic. If you use a soft cloth gaming mousemat, slide a hardcover notebook or wooden coaster under your hands so your strikers can sing.

## 2. How to Hold Your Strikers (Zero Arm Fatigue)

The fastest way to quit a new input method is shoulder soreness. Never hover your hands in mid-air like you're playing an invisible keyboard. Use the **Desk-Anchored Pinch**:

1. **Plant the heels of both palms flat on the table** right in front of your laptop. Your wrists stay relaxed and supported the entire time.
2. **Pinch each object lightly between your thumb and index finger**—just like holding a guitar pick—with about half of the object extending below your fingertips to touch the desk.
3. **Hold near the edge, never the dead center.** Squeezing the middle of a coin or spoon muffles its ring like grabbing a bell. Holding near the rim lets the metal vibrate freely.
4. **Keep gestures compact.** A **1-inch line** or a **dime-sized circle** gives the microphone more than enough acoustic detail. Small movements mean high speed and zero fatigue.

---

## 3. Quick Start & Calibration

If you use Nix, launch CoinChord directly from the project directory:

    nix run

Or enter the development shell and run with Go:

    nix develop
    go run main.go

*(Linux uses `arecord` from `alsa-utils`; macOS uses `rec` from `sox`—both are automatically provided by `flake.nix`.)*

### Step 1: Calibrate to Your Room & Objects
When the interactive menu opens, choose **`1) Setup & Calibrate`**:
1. **Stay still for 2 seconds** while CoinChord listens to your room's baseline noise (fans, AC, street hum) and sets a smart volume trigger.
2. **Tap your Left object 5 times** at a relaxed, natural volume.
3. **Tap your Right object 5 times.**
4. **Draw a straight Line scrape 5 times** (~1 inch across the table).

CoinChord saves your personal acoustic profile to **`~/.coinchord.json`**. Look at your Left and Right pitch readout: a gap of **1,500 Hz or more** means effortless left/right accuracy.

## 4. The Four Acoustic Shapes

CoinChord reads both **which hand** moved (via pitch) and **what geometry** you drew on the desk (via duration, friction flatness, pressure ripples, and pitch sweep):

* **`TAP` (`L_TAP`, `R_TAP`, `DUAL_TAP`):** A crisp, quick vertical tap (`<= 125 ms`). Dropping both hands at once triggers `DUAL_TAP`.
* **`LINE` (`L_LINE`, `R_LINE`):** A straight, steady ~1-inch drag across the desk with even pressure and flat pitch.
* **`CURV` (`L_CURV`, `R_CURV`):** A curved hook or J-stroke. As your wrist arcs, the contact angle shifts and sweeps the pitch centroid by `> 480 Hz`.
* **`CIRC` (`L_CIRC`, `R_CIRC`):** A small circular swirl on the desk (`> 340 ms` or 2+ rhythmic speed ripples).
* **`DUAL_SHAPES`:** Both hands drawing continuous shapes at the same time (for example, Left draws a straight line while Right draws a curve), blending high-band and low-band friction simultaneously.

---

## 5. Your 5-Day Learning Path (From First Tap to Whole Phrases)

Don't rush. Ten focused minutes a day builds permanent muscle memory:

* **Day 1 — The Single-Hand Taps & Lines:**
  Practice `L_TAP` (`"the "`), `R_TAP` (`"and "`), `DUAL_TAP` (`"I "`), `L_LINE` (`"to "`), and `R_LINE` (`"of "`). Focus on keeping your wrists planted and tapping lightly.
* **Day 2 — Curves & Circles:**
  Add `L_CURV` (`"you "`), `R_CURV` (`"for "`), `L_CIRC` (`"in "`), and `R_CIRC` (`"that "`). Notice how a hook feels noticeably different in your wrist than a straight line or a swirl.
* **Day 3 — Two-Hand Rhythms:**
  Combine two gestures within **360 milliseconds** (`chord_win_ms`) so they fuse into one word:
  * `L_LINE + R_CURV` -> **`with `** (Left slides straight, Right hooks right after)
  * `L_CURV + R_LINE` -> **`from `** (Left hooks, Right slides straight)
  * `L_CIRC + R_TAP` -> **`this `** (Left swirls, Right punctuates with a tap)
  * `R_CIRC + L_TAP` -> **`have `** (Right swirls, Left taps)
  * `DUAL_SHAPES` -> **`something `** (Both hands glide simultaneously)
* **Day 4 — Whole-Phrase Drills (`Option 2`):**
  Open **`2) Whole-Word & Shape Trainer`**. Instead of boring letter drills, CoinChord feeds you full English sentences (`"the and to of in that you for "`), prompts the shape chord for each word, and calculates your live **WPM** when you finish the phrase. Say each word out loud as you draw its shape to lock in the neural link.
* **Day 5 — Programming Mode:**
  Perform **`L_CIRC + R_CIRC`** (two circular swirls) anytime to switch into **Code Mode**, where `L_TAP` = `"def "`, `R_TAP` = `"return "`, `DUAL_TAP` = `"()"`, `L_LINE` = `" = "`, `R_LINE` = `":\n    "`, `L_CIRC` = `"for "`, `R_CIRC` = `" in "`, `L_CURV` = `"if "`, and `R_CURV` = `"else:\n    "`. Swirl both hands again (`L_CIRC + R_CIRC`) to jump right back to English prose.


## 6. Customizing `~/.coinchord.json`

Every chord, timing window, and training phrase lives in **`~/.coinchord.json`**, which you can edit with any text editor—no recompiling required:

* **Add Custom Words or Snippets:** Add any key to `"prose_chords"` or `"code_chords"` using single shapes (`"L_CURV"`) or two-step combos (`"L_LINE+R_CURV"`, `"R_TAP+L_CIRC"`, etc.).
* **Tune Your Chord Speed (`"chord_win_ms"`):** Default is `360` ms. Bump it up to `450` ms during your first few days so two-hand combos never split into separate words; drop it to `240` ms once your hands get fast.
* **Add Custom Practice Sentences (`"word_phrases"`):** Add your own sentences to `"word_phrases"` in `~/.coinchord.json`, and the Interactive Trainer will automatically look up the matching shape chords and time your WPM.

---

## 7. Quick Troubleshooting & Reference

* **False triggers from speaking or background noise:** CoinChord filters out everything below `1,800 Hz` (so voices and bass hum are ignored), but if loud clacks trigger inputs, open `~/.coinchord.json` and raise `"trigger"` slightly (e.g., from `0.032` to `0.050`).
* **Curves (`CURV`) registering as straight lines (`LINE`):** Exaggerate the wrist hook slightly at the end of your stroke so the coin's tilt changes cleanly.
* **Circles (`CIRC`) registering as lines (`LINE`):** Make your swirl slightly wider or complete 1.5 rotations so at least two speed/pressure ripples occur.

### Quick Menu Reference
* **`1) Setup & Calibrate`** — Measures ambient noise and calibrates Left pitch, Right pitch, and desk friction.
* **`2) Whole-Word & Shape Trainer`** — Guided whole-word phrase practice with real-time WPM scoring.
* **`3) Live Prose Mode`** — Real-time English word-chording output (`L_CIRC+R_CIRC` toggles to Code Mode).
* **`4) Live Code Mode`** — Real-time programming syntax output (`L_CIRC+R_CIRC` toggles to Prose Mode).

