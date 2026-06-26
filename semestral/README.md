# Go Messenger

Projekt implementuje distribuovanou chatovací aplikaci v jazyce Go.

Systém je rozdělen na tři samostatné spustitelné programy:
* `profile-server` - správa uživatelských účtů, autentizace a profilů
* `messaging-server` - přenos zpráv a správa chatů
* `client` - terminálový klient postavený nad knihovnou Bubble Tea

Komunikace mezi všemi komponentami probíhá pomocí gRPC přes TLS.

---

## 1. Implementované funkce

### Povinná část
* **Registrace uživatelů**: Generování a využití unikátního 9místného `UserID` (viz `internal/profiles`).
* **Autentizace a přihlášení**: Přihlášení pomocí UserID a hesla. Bezpečné ukládání hesel pomocí `bcrypt` hashů na straně profilového serveru.
* **Správa tokenů (JWT)**: Autentizace pomocí asymetricky podepsaných JWT tokenů (`RS256`). Využití dvojice krátkodobého access tokenu (platnost 1 hodina) a dlouhodobého refresh tokenu (platnost 7 dní) (viz `internal/auth`).
* **Reálný čas přes gRPC**: Odesílání zpráv pomocí standardních RPC a okamžité asynchronní doručování online uživatelům přes jednosměrný Push stream (`Subscribe`).
* **Store-and-Forward (Offline zprávy)**: Ukládání nedoručených zpráv pro offline uživatele do serverové SQLite databáze (viz `internal/messaging`).
* **Lokální historie**: Kompletní ukládání historie zpráv v lokální databázi na straně klienta.
* **Znovupřipojení**: 
  * Automatická obnova gRPC streamů a připojení po výpadku sítě (viz `internal/client/tui` a `tui/models/chat_model.go`).

### Volitelné funkce
* **Presence**: Sledování stavů online/offline a podpora režimu neviditelnosti (**Invisible Mode**).
* **Skupinové chaty**: Vytváření skupin, příhlasení členů, odhod členu.
* **Potvrzení o doručení a přečtení**: Sledování životního cyklu zprávy (doručení a zobrazení u příjemců).

---

## 2. Architektura systému

## Profilový server

Profilový server spravuje uživatelské účty, autentizaci a veřejné informace o uživatelích.
Při registraci je heslo zahashováno pomocí algoritmu bcrypt a do databáze je
uložen pouze výsledný hash.

### Poskytované operace

- registrace nového uživatele,
- přihlášení pomocí `UserID` a hesla,
- obnovení access tokenu pomocí refresh tokenu,
- získání profilu uživatele,
- aktualizace profilu,
- ověření existence více uživatelů najednou,
- změna online stavu uživatele.

### Autentizační tokeny

Po úspěšném přihlášení server vytvoří dvojici JWT tokenů podepsaných
algoritmem RS256:

- krátkodobý access token 1h,
- dlouhodobější refresh token 7d.

Access token klient používá při komunikaci s oběma servery.
Messaging server ověřuje podpis tokenu pomocí veřejného RSA klíče, aniž by
znal uživatelské heslo. 

Refresh tokeny jsou jednorázové. Při vytvoření refresh tokenu se jeho identifikátor (`jti`) uloží do mapy
aktivních tokenů. Při použití tokenu server ověří jeho platnost a následně
jej ze seznamu odstraní. Opakované použití stejného tokenu je odmítnuto.

Seznam aktivních refresh tokenů je uložen pouze v paměti procesu
(`internal/profiles/service.go`). Po restartu serveru je proto nutné
provést nové přihlášení.

## Messaging Server (`messaging-server`)
Zajišťuje dočasnou perzistenci a distribuci zpráv prostřednictvím vestavěného pub/sub brokeru (`internal/messaging/broker.go`). Server neuchovává kompletní historii doručených zpráv, ale spravuje transakční vyřizování front.

### Životní cyklus a sekvence operací na Messaging Serveru

Messaging server funguje na principu Store-and-Forward a kombinace synchronního gRPC 
volání pro odesílání a jednosměrného Push streamu pro příjem zpráv.
Níže je popsáno, co přesně se na serveru
děje při připojení klienta a odeslání zprávy.

### 1. Přihlášení k odběru zpráv (Subscribe)
Jakmile se klient připojí, volá RPC metodu `Subscribe`. Tato operace inicializuje gRPC
stream směrem od serveru k uživateli:

1. **Vytvoření gRPC streamu**: Spustí se jedna dlouhožijící gorutina, která plně vlastní tento stream až do odpojení klienta.
2. **Registrace v Brokeru**: Gorutina vytvoří v in-memory Pub/Sub vrstvě (`internal/messaging/broker.go`) objekt
   `UserSession` s buferovaným kanálem (`messageChan`).
3. **Propagace stavu (Presence)**:
    * Pokud **není** nastaven parametr `IsInvisible`, server zavolá `profiles.ProfileService` a označí uživatele jako **online**.
    * Pokud klient nastavil `IsInvisible`, propagace stavu se přeskočí a uživatel navenek zůstává jako **offline** (přestože zprávy normálně přijímá i odesílá).
4. **Odeslání offline zpráv**: Rutina se dotáže do databáze (SQLite) na všechny dosud nedoručené zprávy pro daného uživatele a okamžitě je odešle přes stream.
5. **Čekání na události**: Rutina přejde do stavu čekání (sleep/select),
   kde naslouchá buď novým zprávám z brokeru, nebo signálu o ukončení session (odpojení klienta / shození brokerem).

6. **Odpojení (Clean up)**: Při ukončení streamu (`defer`) se uživatel v `ProfileService` opět označí jako **offline** (pokud nebyl invisible).

> **Důležité**: Pokud je klient příliš pomalý a jeho kanál v brokeru se přeplní,
broker jeho session z bezpečnostních důvodů ukončí (`killing slow client`). Klient se musí znovu připojit.
Žádné zprávy se tím neztratí, protože jsou stále uloženy v DB serveru.

---

### 2. Odeslání a distribuce zprávy (SendMessage & Multiplexing)
Když uživatel odešle zprávu, volá se RPC metoda `SendMessage`:

1. **Ověření oprávnění**: Server vytáhne JWT claims z kontextu, zjistí ID odesílatele a v databázi ověří, zda je odesílatel skutečně členem daného chatu (`GetChatMembers`). Pokud ne, vrací `PermissionDenied`.
2. **Vytvoření potvrzení (Acks)**: Server vygeneruje záznamy o doručení (`MessageAck`) pro všechny ostatní členy chatu (vyjma odesílatele). Pokud je odesílatel v chatu sám (např. poznámky pro sebe), zpráva se neukládá a rovnou se vrací úspěch.
3. **Uložení do DB**: Zpráva a její příslušné `Acks` záznamy se v rámci jedné databázové transakce (`InsertMessageWithAcks`) zapíšu do SQLite.
4. **Multiplexing v Brokeru**: Server asynchronně v nové gorutině zavolá `go s.broker.Publish(&message, acks)`.
   Broker pod zámkem (`sessionsMu.RLock`) vyhledá **všechny aktivní session** pro všechny cílové uživatele.
   Pokud má jeden uživatel připojených více klientů současně, zpráva je multiplexována do kanálů **všech těchto aktivních klientů**.

---

### 3. Potvrzení o doručení, přečtení a mazání (AckMessage)
Server neuchovává kompletní historii chatu pro všechny uživatele, stará se primárně o doručení "online" a uložení "offline" zpráv.

1. **Doručení na klienta**: Jakmile zpráva projde streamem a klient ji úspěšně přijme a zpracuje (případně vykreslí uživateli), pošle zpět na server RPC volání `AckMessage`.
2. **Odstranění ze serveru**: Na základě tohoto potvrzení server aktualizuje stav daného `MessageAck` v databázi.
3. **Smazání zprávy**: Jakmile potvrdí doručení **poslední** příjemce, pro kterého byla zpráva určena (tzn. pro danou zprávu již neexistuje žádný nevyřízený `Ack`),
  server zprávu z databáze smaže. Ack objekty zustanu pro sledovani stavů zpravy uzivatelem (jestli byla prectena a pod)

---

## 3. Klientská část a TUI Architektura

Klientská aplikace využívá terminálové uživatelské rozhraní postavené na frameworku **Bubble Tea** (architektura Elm: *Model-Update-View*) a
knihovně **Lip Gloss**.

### Hierarchie a komunikace modelů
UI je modulární, rozdělené na hlavní kořenový model a izolované submodely, které komunikují pomocí zpráv (`tea.Msg`):

* **`root_model.go`**: Kořenový uzel aplikace. Zapouzdřuje sdílený stav (`AppContext`), směruje systémové události a provádí přepínání kontextů (např. přesun z `onboarding_model.go` do hlavní aplikace, zobrazení `error_model.go` při fatální chybě).
* **Kompozice chatovací obrazovky (`chat_model.go`)**: Hlavní pracovní prostor. Skládá se ze tří koordinovaných submodelů:
  * `chat_list_model.go` - interaktivní seznam dostupných konverzací.
  * `messages_list_model.go` - viewport pro renderování historie zpráv.
  * `messages_input_model.go` - textový editor pro psaní nových zpráv.
* **Transientní stavy (Modální dialogy)**: Specifické akce jako vytvoření chatu (`create_chat_submodel.go`), pozvání uživatele (`invite_user_model.go`), opuštění skupiny (`leave_chat_submodel.go`) nebo zobrazení podrobností o doručení (`message_ack_submodel.go`) jsou implementovány jako vnořené podstavy, které dočasně přebírají focus klávesnice.

### Detailní chování specifických funkcí
* **Presence & Invisible Mode**: Při přihlášení k odběru předává klient příznak `IsInvisible`. 
* Server v tomto případě potlačí zápis do `ProfileService`. Kdyz uzivatel meni `IsInvisible`, klient se odpoji a pripoji znovu.
* **Skupinové chaty**: Zahrnují operace vytvoření, přidání člena, opuštění a zobrazení informací o skupině.
* **Sledování Message ACKs**:
  * `delivered_at`: Klient po přijetí zprávy ze streamu provede lokální zápis do SQLite a poté odesílá potvrzení serveru, který zprávu označí za doručenou.
  * `read_at`: Pokud má uživatel aktivně vybrané okno daného chatu v `messages_list_model.go` a zpráva se zobrazí na obrazovce,
   klient odešle gRPC požadavek na zápis přečtení. Ucasnici chatu zprávy můžou tyto časy videt. V pripade `IsInvisible` neposila se zprava o precteni.

---

### Spuštění projektu
```bash
# 1. Kompilace všech komponent
./scripts/build.sh

# 2. Spuštění serverovů
./scripts/profile-server.sh
./scripts/messaging-server.sh

# 3. Spuštění nezávislých klientských instancí
./scripts/client.sh c1 # Instance 1: data v data/c1, logy v logs/c1
./scripts/client.sh c2 # Instance 2: data v data/c2, logy v logs/c2
```

## Struktura projektu
- `cmd/` – vstupní body aplikace (`client`, `messaging-server`, `profile-server`).
- `internal/auth/` – JWT tokeny, RSA klíče a gRPC middleware.
- `internal/messaging/` – logika chatů, broker pro správu aktivních spojení a SQLite repozitář.
- `internal/profiles/` – správa profilů, autentizace a online stav uživatelů.
- `internal/client/` – klientská logika:
  - `state/` – globální kontext aplikace a session cache,
  - `storage/` – lokální SQLite databáze klienta,
  - `tui/` – terminálové uživatelské rozhraní.
- `protos/` – definice gRPC služeb.
- `generated/` – automaticky vygenerovaný Go kód z `.proto` souborů.
- `resources/` – TLS certifikáty a JWT klíče.
- `scripts/` – build a spouštěcí skripty.