# past

a terminal ui for [`pass`](https://www.passwordstore.org/), the standard unix
password manager — with first-class github integration.

`past` (pass + tui) wraps your existing `pass`, `git`, `gpg`, and `gh` installs.
it never re-implements crypto and never stores secrets itself: every operation
shells out to the tools you already trust. clipboard copies go through `pass -c`,
so pass owns the auto-clear timeout.

## features

- browse the store as a collapsible tree (entries nested under their folders), with a detail pane (password masked by default)
- copy, generate, regenerate, add, edit, rename/move, duplicate, and delete entries
- bulk-import from a CSV or XML export: `past import secrets.csv`
- every change auto-commits by default; turn it off (`auto_commit = false`) to stage changes and commit them yourself from the github menu (`h`)
- git sync status in the bar (`↑ahead ↓behind`, dirty dot) and one-key `pull --rebase` + `push`
- github menu: provision a private store repo via `gh`, view secret-scanning alerts and recent actions runs

## import

bring entries in from another manager's export, non-interactively:

```sh
past import secrets.csv     # or secrets.xml
```

- **csv** — a header row with a required `name` (or `path`) column and an
  optional `password` column; every other non-empty cell becomes a `key: value`
  line, e.g. `name,password,user,url`.
- **xml** — `<store><entry name="folder/x"><password>…</password><user>…</user></entry></store>`;
  a `<password>` child is the first line, any other child becomes metadata.

imported entries are written but not committed — review them in the tree and
commit from the github menu (`h`) when you're happy.

## prerequisites

| tool  | needed for            | install                |
|-------|-----------------------|------------------------|
| `pass`| everything            | `brew install pass`    |
| `gpg` | pass encryption       | `brew install gnupg`   |
| `git` | sync / status         | (usually preinstalled) |
| `gh`  | github features only  | `brew install gh`      |

you also need an initialized store and an authenticated `gh`:

```sh
pass init <your-gpg-id>   # creates ~/.password-store (or $PASSWORD_STORE_DIR)
gh auth login             # only for the github menu
```

`past` shows a friendly setup screen — not a crash — when a prerequisite is missing.

## install

```sh
go build -o past .
./past
```

`past` honours `$PASSWORD_STORE_DIR` (default `~/.password-store`).

## keybindings

| key       | action                                  |
|-----------|-----------------------------------------|
| `j` / `k` | move down / up                          |
| `/`       | filter entries                          |
| `enter` / `space` | on a leaf: reveal / mask the password; on a folder: fold / unfold |
| `c`       | copy — password (clears after a timeout) for a single-field entry; otherwise pick a field. the password always clears via pass; other field values are copied as-is and don't auto-clear |
| `g`       | new login (generated password), filled in-app |
| `a`       | new entry — pick a type (login/password/credit card/email/server/database/api token/secure note), fill fields in-app |
| `e`       | edit the selected entry (opens `$EDITOR`)|
| `p`       | regenerate the entry's password in place (keeps metadata) |
| `r`       | rename / move the entry (type a new path) |
| `y`       | duplicate the entry to a new path       |
| `d`       | delete (asks `y` to confirm)            |
| `s`       | sync — `pass git pull --rebase` then `push` |
| `h`       | github menu (commit, sync, repo)        |
| `u`       | unlock — enter the gpg passphrase in-app (also prompts automatically on a locked key) |
| `?`       | toggle help                             |
| `q`       | quit                                    |

inside the github menu: commit staged changes, sync, provision a repo, and view
alerts / actions runs. `j`/`k` move, `enter` selects, `esc` backs out.

## config

optional dotfile at `~/.config/past/config` (override with `$XDG_CONFIG_HOME` or
`$PAST_CONFIG`). simple `key = value` lines, `#` comments. missing file or keys
fall back to defaults.

```ini
# commit every change automatically (default: true)
auto_commit = false
```

## a note on secret-scanning

the store holds *encrypted* `.gpg` blobs, so github secret-scanning will almost
always report **no alerts** — the value of the panel is confirming the repo is
clean and private, not catching leaked plaintext.

## non-goals (v1)

otp/totp, multiple stores, mouse support, and non-github remotes. the git layer
is remote-agnostic, but only github gets the `gh`-powered features.
