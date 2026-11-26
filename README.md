# CoinChord: type and drive your computer with two coins and your laptop mic

[![Sponsor](https://img.shields.io/badge/sponsor-vic-ea4aaa?logo=githubsponsors)](https://github.com/sponsors/vic)

![genesis](./genesis.gif)

CoinChord turns the microphone in your laptop into a chording keyboard. You hold a small object in each hand — two coins, a key and a spoon, even just your fingernails — and you tap, drag, hook and swirl them on your desk. CoinChord listens, tells your hands apart by their sound, tells the shapes apart by how they ring and scrape, and turns each little motion into a whole word, a shortcut, a scroll or a mouse click. No special hardware, no camera, no batteries.

## The idea: stop spelling, start conducting

A normal keyboard makes you hunt for one letter at a time. Chording keyboards like the CharaChorder go much faster because one motion types a whole word. CoinChord brings that idea to the desk you already have.

Think like a drummer, not a typist. A light tap with your left coin can mean "the". A tap with the right coin can mean "and". A small knock can type "once upon a time, " in one go. Two gestures close together make a chord, and chords give you hundreds of words. You never spell "w-i-t-h" again: your hands simply learn that one motion means "with".

Your mic is a surprisingly good ear. It hears 44,100 samples a second, so it can tell a bright "tink" from a deep "clink", and a crisp tap from a long scrape, a hooked curve or a circle.

## Pick your strikers

CoinChord only needs things that sound different. A bright, light object goes in the left hand and a deeper, heavier one in the right. These all work well:

| Kit | Left hand | Right hand | Why people like it |
| :--- | :--- | :--- | :--- |
| Pocket coins | Small coin | Large coin | Smooth edges glide into curves and circles, and they ring clearly. |
| Coins on a plate | Small coin | Large coin | Tap on an upside-down ceramic plate: loud and bright. |
| Key & spoon | House key | Metal spoon | So different that even a noisy café can't confuse them. |
| Desk pens | Plastic pen cap | Metal pen | Perfect if you already fidget with pens. |
| Nail & ring | Fingernail flick | Ring or knuckle | Nothing to carry: it's always on you. |
| Pick & mug | Guitar pick | Mug rim or coaster | Effortless, fast taps and sweeps. |
| Kitchen four, Desk four | Two objects | Two objects | Four voices for twice the chords. |
| Ten nails | Five fingernails | Five fingernails | Scratch the laptop palm rest: the most chords of all. |

You can also build your own kit with as many objects per hand as you like, and save it.

Hard surfaces sing best: wood, laminate, glass, stone, a ceramic plate or a hardcover book. On a soft mouse mat, slide a notebook under your hands.

## Hold them like this

Rest the heels of both palms on the table, so your arms never get tired. Pinch each object lightly near its edge, like a guitar pick, with part of it touching the desk. Holding the middle muffles the ring. Keep it small: a one-inch line or a dime-sized circle is plenty, and small motions are fast motions.

## Get it running

Grab the binary for your system from the GitHub releases page. Linux, macOS and Windows builds are single static files: download, unpack, run. CoinChord starts a small web server on your own computer and opens its page in your browser, at http://127.0.0.1:7317. Only your computer can reach it.

With Nix, run it straight from the repo:

    $ nix run

A few flags come in handy:

    $ coinchord -output on          # chords act on your computer right away
    $ coinchord -open=false         # don't open the browser
    $ coinchord -listen 127.0.0.1:8080
    $ coinchord -text mytext.txt    # practice with your own text

On Linux, CoinChord listens through `arecord` from alsa-utils. On macOS it listens through `rec`, which you get with `brew install sox`. On Windows it listens directly.

### Run it as a service

On NixOS, add the flake and switch it on. CoinChord then starts with your desktop session, and the users you list can send keyboard and mouse input:

    services.coinchord = {
      enable = true;
      users = [ "alice" ];
      output = "off";   # or "on"
      port = 7317;
    };

With home-manager, use `homeManagerModules.default` and the same `services.coinchord` options (without `users`). Open http://127.0.0.1:7317 whenever you want to see what CoinChord hears.

## A quick tour

Everything on the page is a button or a slider, so you never need to remember a key. The bar at the top always tells you what to do next and gives you a button for it, and the little path above it shows how far you've come.

Right under it, the live strip is always there. Your objects appear as coins that light up when the mic hears them, the mic status says if sound is coming in, and a level bar shows your room noise (yellow) next to the trigger line (red). A sound has to cross the red line to count. If the mic is muted or too loud, the strip tells you and offers a button to fix it.

**Kits** shows your kit and lets you rename, add and remove objects, save the whole setup, and switch between saved setups. Below are kit ideas to start from.

**Calibrate** teaches CoinChord your objects. Stay quiet for two seconds, tap each object five times, drag one three times, and then try every gesture. You see live which ones the mic hears well, and a report at the end tells you how many chords you have at your fingertips.

**Gestures** shows every gesture your kit can make, with a little drawing and a how-to. Each card lights up when you perform it, and the side panel shows how CoinChord measured your last stroke and which object it matched.

**Tutor** starts with a bit of magic: one gesture types a whole phrase, and a handful type a full sentence. Then you work through short stories sentence by sentence. The tutor always shows the next chord, how to make it and what it types, and it tracks your words per minute, chords per minute and accuracy. Mistakes never type anything, so there's no pressure. Drop your own `.txt` files in `~/.config/coinchord/stories` to practice with them.

**Layers** shows what every chord does. Click any row to try it in the sandbox, or perform chords and watch them type.

**Settings** lets you tune detection by dragging lines on live graphs, record your own chords, and manage your practice texts.

## The gestures

Every gesture is a striker plus a shape. The shapes are:

* **TAP**: a light, quick tap.
* **KNOCK**: a firmer tap, about twice as loud.
* **LINE**: a straight one-inch drag.
* **CURV**: a drag that hooks at the end, like a J.
* **CIRC**: a small circle, one and a half turns.
* **SLIDE**: a long, slow drag.
* **ZIG**: a quick zigzag scribble.

You can also sound both hands at once, which CoinChord writes as `L&R`. Two gestures close together make a chord, so two coins already give you 21 gestures and 462 chords.

## Layers

A layer is a set of chords for one job, and you can hop between layers with two-hand gestures at any time.

`english` has the most common words and phrases. `alpha` spells letter by letter, `num` has digits and `sym` has punctuation. `kbd` has Return, Backspace, Tab, Escape and raw shortcuts. `spanish`, `python` and `go` are there too, and you can add your own.

## Drive your whole computer

Flip the **Output** switch at the bottom left and CoinChord becomes a real keyboard and mouse for every window on your computer. Words land in your editor, shortcuts switch windows, circles scroll pages and knocks click. Flip it again to stop.

Three layers are made for this, and they share one easy motion map: your left hand goes left or up, your right hand goes right or down. A tap is a small step, a line moves up or down, a slide goes far, a curve jumps a page and a circle scrolls.

* `nav` moves the text cursor by characters, words, lines and pages, and selects, copies and pastes.
* `window` switches, maximizes, minimizes and snaps windows, moves between desktops, opens the launcher and steps through tabs.
* `mouse` is a mouse pad: nudge and jump the pointer, click, double-click, right-click, drag and scroll.

Window commands work the same everywhere: the "next window" chord presses Alt+Tab on Linux and Windows and Cmd+Tab on a Mac.

The Tutor has five host-skill drills — Windows, Navigation, Scrolling, Mouse pad, and Tabs & browser — that walk you through all of it, one action at a time.

The first time, each system wants a quick permission:

* **Linux:** CoinChord creates a virtual keyboard and mouse, which works on both X11 and Wayland. Give your user access to `/dev/uinput`, for example with the udev rule `KERNEL=="uinput", GROUP="input", MODE="0660"` and by joining the `input` group.
* **macOS:** allow CoinChord (or the terminal you start it from) in System Settings → Privacy & Security → Accessibility.
* **Windows:** nothing to do.

## Make it yours

In **Settings → Your chords**, click *Record a chord*, perform any chord, and type what it should do. You can also click any row to change it. An expansion is plain text mixed with tokens in braces:

* keys: `{enter}`, `{bs}`, `{tab}`, `{space}`, `{esc}`, arrows like `{left}`, and combos like `{ctrl+alt+left}`
* layers: `{layer:kbd}` switches layer, `{undo}` takes back the last chord
* commands: `{do:switch}`, `{do:snap-left}`, `{do:copy}`, `{do:tab-new}` and friends
* mouse: `{move:40,0}`, `{scroll:3}`, `{click:left}`, `{double:left}`, `{down:left}`, `{up:left}`
* a literal brace: `{{`

**Settings → Detection** shows your loudness live on a graph. Drag the red line to choose how loud a sound must be to count: above the yellow room noise, below your taps. Under it, a slider for each border between shapes — tap length, knock strength, curve bend, slide length, circle and zigzag bumps, two-hand balance, chord window — with a dot that shows where your last stroke landed, so you can see exactly which side of the line it fell on.

On the **Kits** page you can save the whole setup — calibration, scores and chords — so you can switch between your coins-on-the-desk setup and your nails-on-the-laptop setup with one click.

Your profile lives in `~/.config/coinchord/profile.json`.

## When things don't click

* **Nothing lights up:** look at the mic status in the live strip. If it says the mic sends silence, click *Unmute*. If it says too loud, click *Volume −* until the yellow room-noise mark sits well left of the red line.
* **Noise or talking types words:** drag the red line up in Settings → Detection, or click *Set from room noise* while the room is quiet.
* **Curves come out as lines:** hook your wrist a little more at the end.
* **Circles come out as lines:** make one and a half turns, a bit wider.
* **Your two objects get mixed up:** the calibration report shows how alike they sound. Above 90% alike, try objects that sound more different.
* **Want to test without a mic:** `coinchord -input file.raw` plays raw 16-bit mono 44,100 Hz audio from a file.

## Help make detection better

Start CoinChord with `-record DIR` and calibrate as usual. It saves what the mic heard together with what you were asked to do. Later, `coinchord -eval DIR/<session>.raw` replays that session and tells you how many gestures were detected right and which ones got mixed up — handy when you tune the sliders, and gold if you want to share recordings to improve CoinChord.

## Support

If CoinChord makes you smile, consider [sponsoring vic](https://github.com/sponsors/vic) — there's a ♥ Sponsor button right in the app too.
