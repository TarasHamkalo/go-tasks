# Go Messenger

Projekt implementuje distribuovanou chatovací aplikaci v jazyce Go.

Systém je rozdělen na tři samostatné spustitelné programy:

* `profile-server` - správa uživatelských účtů, autentizace a profilů
* `messaging-server` - přenos zpráv a správa chatů
* `client` - terminálový klient postavený nad knihovnou Bubble Tea

Komunikace mezi všemi komponentami probíhá pomocí gRPC přes TLS.

## Implementované funkce

### Povinná část
* Registrace uživatelů, využití unikátního 9místného UserID
* Přihlášení pomocí UserID a hesla
* Bezpečné ukládání hesel pomocí bcrypt
    - pro vyše spomenute viz `internal/profiles`
* Autentizace pomocí JWT tokenů (RS256) + obnovování pomocí refresh tokenů
    - viz balík `internal/auth`
* Odesílání zpráv v reálném čase pomocí gRPC streamu
* Ukládání nedoručených zpráv pro offline uživatele
    - viz balik `internal/messaging`
* Lokální ukládání historie zpráv na klientovi
* Automatické obnovení spojení po výpadku
    - viz balik `internal/client/tui` + konkretně `tui/models/chat_model.go`

### Volitelné funkce
* Presence (online/offline + invisible mode)
* Skupinové chaty
* Potvrzení o doručení a přečtení zpráv

# Architektura systému
## Profilový server

Profilový server spravuje uživatelské účty, autentizaci a veřejné informace o uživatelích.

### Poskytované operace

- registrace nového uživatele,
- přihlášení pomocí `UserID` a hesla,
- obnovení access tokenu pomocí refresh tokenu,
- získání profilu uživatele,
- aktualizace profilu,
- ověření existence více uživatelů najednou,
- změna online stavu uživatele.

### Bezpečnost a hesla

Hesla nejsou nikdy ukládána v otevřené podobě. Při registraci je heslo
zahashováno pomocí algoritmu bcrypt a do databáze je uložen pouze výsledný hash.

### Autentizační tokeny

Po úspěšném přihlášení server vytvoří dvojici JWT tokenů podepsaných
algoritmem RS256:

- krátkodobý access token,
- dlouhodobější refresh token.

Access token klient používá při komunikaci s oběma servery.
Messaging server ověřuje podpis tokenu pomocí veřejného RSA klíče, aniž by
znal uživatelské heslo.

### Mechanismus refresh tokenů

Refresh tokeny jsou jednorázové.

Při vytvoření refresh tokenu se jeho identifikátor (`jti`) uloží do mapy
aktivních tokenů. Při použití tokenu server ověří jeho platnost a následně
jej ze seznamu odstraní. Opakované použití stejného tokenu je odmítnuto.

Seznam aktivních refresh tokenů je uložen pouze v paměti procesu
(`internal/profiles/service.go`). Po restartu serveru je proto nutné
provést nové přihlášení.

## Životní cyklus a sekvence operací na Messaging Serveru

Messaging server funguje na principu Store-and-Forward a kombinace synchronního gRPC volání pro odesílání a 
jednosměrného Push streamu pro příjem zpráv. Níže je popsáno, co přesně se na serveru 
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
3. **Uložení do DB**: Zpráva a její příslušné `Acks` záznamy se v rámci jedné databázové transakce (`InsertMessageWithAcks`) zapíší do SQLite.
4. **Multiplexing v Brokeru**: Server asynchronně v nové gorutině zavolá `go s.broker.Publish(&message, acks)`. 
Broker pod zámkem (`sessionsMu.RLock`) vyhledá **všechny aktivní session** pro všechny cílové uživatele. 
Pokud má jeden uživatel připojených více klientů současně, zpráva je multiplexována do kanálů **všech těchto aktivních klientů**.

---

### 3. Potvrzení o doručení, přečtení a mazání (AckMessage)
Server neuchovává kompletní historii chatu pro všechny uživatele, stará se primárně o doručení "online" a uložení "offline" zpráv.

1. **Doručení na klienta**: Jakmile zpráva projde streamem a klient ji úspěšně přijme a zpracuje (případně vykreslí uživateli), pošle zpět na server RPC volání `AckMessage`.
2. **Odstranění ze serveru**: Na základě tohoto potvrzení server aktualizuje stav daného `MessageAck` v databázi. 
3. **Smazání zprávy**: Jakmile potvrdí doručení **poslední** příjemce, pro kterého byla zpráva určena (tzn. pro danou zprávu již neexistuje žádný nevyřízený `Ack`), server zprávu i potvrzení z databáze kompletně smaže, čímž se předchází bobtnání databáze.

---

## 1. Architektura TUI (Terminálové UI)

Klientská aplikace je postavena na frameworku **Bubble Tea** (architektura Elm: Model-Update-View) a pro stylování využívá knihovnu **Lip Gloss**.

### Hierarchie a komunikace modelů
UI je modulární a skládá se z hlavního kořenového modelu a specializovaných submodelů:

* **`root_model.go`**: Hlavní mozek TUI. Spravuje globální stav aplikace, drží sdílený `AppContext` a směruje zprávy (`tea.Msg`) do aktivních submodelů. Reaguje na fatální chyby přepnutím na `error_model.go` nebo na úvodní obrazovku přes `onboarding_model.go`.
* **Submodely a kompozice**:
  * **`chat_model.go`**: Zapouzdřuje celou obrazovku chatu. Kombinuje v sobě `chat_list_model.go` (seznam dostupných konverzací), `messages_list_model.go` (historie zpráv) a `messages_input_model.go` (textové pole pro psaní).
  * **Operace a dialogy**: Modely jako `create_chat_submodel.go`, `invite_user_model.go`, `leave_chat_submodel.go` a `message_ack_submodel.go` fungují jako modální transientní stavy (dialogová okna) pro specifické akce.
* **Synchronizace dat (`pull_data_model.go`)**: Na pozadí nebo při startu spouští asynchronní gRPC příkazy přes `requests.go` pro stažení nových dat a následně emituje vnitřní Bubble Tea zprávy pro aktualizaci lokálního stavu.

---

## 2. Logika znovupřipojení (Retry mechanismus)

Vzhledem k tomu, že komunikace s Messaging serverem probíhá přes dlouhožijící gRPC stream (`Subscribe`), klient implementuje robustní politiku pro potlačení výpadků sítě:

1. **Detekce odpojení**: Pokud stream selže nebo vrátí chybu, gRPC klient detekne přerušení spojení.
2. **Lokální režim**: Klient nezhavaruje, ale přepne UI do stavu offline. Uživatel může stále procházet lokální historii uloženou v SQLite.
3. **Automatické reconnect (gRPC Connection Backoff)**: Vnitřní gRPC transport automaticky zkouší znovu navázat TLS spojení s Messaging a Profile servery pomocí exponenciálního backoff algoritmu (postupné zvyšování prodlevy mezi pokusy).
4. **Obnova streamu**: Po obnovení síťové vrstvy klient transparentně znovu zavolá `Subscribe`, stáhne nahromaděné offline zprávy z databáze serveru a plynule pokračuje v reálném čase bez nutnosti restartu aplikace.

---

## 3. Presence (Správa online stavu)

Stav přítomnosti uživatele je perzistentně sledován na Profile serveru a nabývá hodnot `online` nebo `offline`.

* **Standardní chování**: Po úspěšné autentizaci a spuštění streamu nastaví klient stav na `online`. Při regulérním odhlášení nebo ukončení aplikace (přes `defer` v gRPC streamu) se stav automaticky změní na `offline`.
* **Invisible Mode (Režim neviditelnosti)**: 
  * Uživatel si může v konfiguraci navolit neviditelnost.
  * V tomto režimu klient normálně odesílá i přijímá zprávy v reálném čase přes aktivní gRPC stream.
  * Při volání `Subscribe` je však předán příznak `IsInvisible`, který zablokuje propagaci stavu do Profile serveru. Pro všechny ostatní uživatele v systému se tak daný uživatel jeví jako `offline`.

---

## 4. Skupinové chaty

Systém podporuje decentralizované skupinové konverzace s následujícími pravidly:

* **Správa členů**: Uživatel může vytvořit skupinu a následně do ní zvát další uživatele (`invite_user_model.go`). Každý člen má možnost ze skupiny kdykoliv odejít (`leave_chat_submodel.go`). Seznam aktuálních členů lze zobrazit přes `chat_info_submodel.go`.
* **Omezení historie**: Z důvodu ochrany soukromí a integrity dat vidí nově přidaní uživatelé historii zpráv **pouze od okamžiku, kdy byli do skupiny přidáni**. Server filtruje zprávy na základě časového razítka zařazení uživatele do chat entity.

---

## 5. Potvrzení o doručení a přečtení (Message ACKs)

Pro každou odeslanou zprávu generuje server v transakci sadu sledovacích záznamů pro každého příjemce. Klient tyto stavy aktualizuje pomocí asynchronních gRPC volání:

* **`delivered_at` (Doručeno)**: Nastaví se v okamžiku, kdy zpráva projde push streamem do klientské aplikace a klient ji úspěšně zapíše do své lokální SQLite databáze (`internal/client/storage/`). V té chvíli již zpráva nezávisí na serveru.
* **`read_at` (Přečteno)**: Nastaví se, jakmile uživatel v TUI aktivně otevře daný chat a zpráva se prokazatelně vykreslí na obrazovce (v `messages_list_model.go`).
* **Zobrazení odesílateli**: Odesílatel zprávy může v TUI otevřít detail zprávy (`message_ack_submodel.go`), kde vidí přesný čas doručení a přečtení pro každého jednotlivého člena skupiny.

---

## 6. Lokální ukládání dat a spuštění

Každý klient má izolovanou perzistenci a logování, což umožňuje spouštět více instancí na jednom stroji.

### Spuštění projektu
```bash
# 1. Kompilace všech komponent
./scripts/build.sh

# 2. Spuštění serverové infrastruktury
./scripts/profile-server.sh
./scripts/messaging-server.sh

# 3. Spuštění nezávislých klientských instancí
./scripts/client.sh c1 c1   # Instance 1: data v data/c1, logy v logs/c1
./scripts/client.sh c2 c2   # Instance 2: data v data/c2, logy v logs/c2
<!-- Client -->
<!---->
<!-- Klient je terminálová aplikace využívající: -->
<!---->
<!-- Bubble Tea -->
<!-- Lip Gloss -->
<!-- SQLite -->
<!---->
<!-- Klient umožňuje: -->
<!---->
<!-- registraci a přihlášení -->
<!-- zobrazení seznamu chatů -->
<!-- odesílání zpráv -->
<!-- úpravu profilu -->
<!-- zobrazení profilů ostatních uživatelů -->
<!-- vytváření skupin -->
<!-- pozvání uživatelů -->
<!-- opuštění skupiny -->
<!-- zobrazení doručovacích potvrzení -->
<!-- Presence -->
<!---->
<!-- Profile server uchovává stav uživatele: -->
<!---->
<!-- online -->
<!-- offline -->
<!---->
<!-- Po úspěšném přihlášení klient nastaví stav na online. -->
<!-- Při odhlášení nebo ukončení klienta je stav změněn na offline. -->
<!---->
<!-- Uživatel může aktivovat Invisible Mode. V tomto režimu: -->
<!---->
<!-- zprávy jsou normálně přijímány i odesílány, -->
<!-- ostatním uživatelům je zobrazován stav offline. -->
<!-- Skupinové chaty -->
<!---->
<!-- Podporované operace: -->
<!---->
<!-- vytvoření skupiny -->
<!-- přidání člena -->
<!-- odebrání člena -->
<!-- opuštění skupiny -->
<!-- zobrazení členů skupiny -->
<!---->
<!-- Nově přidaní členové vidí zprávy od okamžiku svého přidání. -->
<!---->
<!-- Potvrzení doručení a přečtení zpráv -->
<!---->
<!-- Ke každé zprávě se eviduje záznam pro každého příjemce. -->
<!---->
<!-- Sledované údaje: -->
<!---->
<!-- delivered_at – klient úspěšně zprávu uložil lokálně -->
<!-- read_at – klient zprávu zobrazil -->
<!---->
<!-- Odesílatel může zobrazit detailní stav zprávy pro všechny příjemce. -->
<!---->
<!-- Ukládání dat -->
<!---->
<!-- Pro perzistenci je použita SQLite. -->
<!-- Spuštění projektu -->
<!-- Build -->
<!-- ./scripts/build.sh -->
<!-- Spuštění serverů -->
<!-- ./scripts/profile-server.sh -->
<!-- ./scripts/messaging-server.sh -->
<!-- Spuštění klienta -->
<!-- ./scripts/client.sh c1 c1 -->
<!-- ./scripts/client.sh c2 c2 -->
<!---->
<!-- První argument určuje adresář pro lokální data, druhý adresář pro logy. -->
<!---->
<!-- Konfigurace -->
<!---->
<!-- Všechny komponenty používají konfiguraci přes proměnné prostředí. -->
<!---->
<!-- Konfigurovat lze například: -->
<!---->
<!-- porty serverů -->
<!-- cesty k databázím -->
<!-- cesty ke klíčům a certifikátům -->
<!-- JWT issuer -->
<!---->
<!-- Ukázkové hodnoty jsou nastaveny ve skriptech v adresáři scripts/. -->
<!-- == 4. Структура Директорій --> -->
<!---->
<!-- * `cmd/` - Вхідні точки для компіляції (`client`, `messaging-server`, `profile-server`). --> -->
<!-- * `internal/auth/` - Логіка JWT, ключі (RSA) та gRPC middleware (interceptors). --> -->
<!-- * `internal/messaging/` - Логіка чатів: Broker (управління сесіями), сервісний шар та SQLite репозиторій. --> -->
<!-- * `internal/profiles/` - Бізнес-логіка профілів, валідація, генерація ID та управління статусами. --> -->
<!--  * `internal/client/` - Логіка клієнта: --> -->
<!--    - `state/` - Глобальний контекст, конфігурація та підключення. --> -->
<!--    - `storage/` - Локальна БД клієнта для кешування історії. --> -->
<!--    - `tui/` - Візуальні компоненти термінального інтерфейсу (моделі, стани екранів). --> -->
<!--  * `data/` - Файли баз даних SQLite для серверів та індивідуальних клієнтів. --> -->
<!--  * `protos/` та `generated/` - gRPC контракти (`.proto`) та згенерований Go код. --> -->
