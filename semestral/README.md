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

<!-- ### Kde co v projektu najít (Kód) -->
<!---->
<!-- * **`internal/messaging/service.go`**: Obsahuje samotnou gRPC implementaci metod `Subscribe` (správa jednosměrného streamu a presence) a `SendMessage` (validace, ukládání a spouštění distribuce). -->
<!-- * **`internal/messaging/broker.go`**: Implementuje in-memory pub/sub vrstvu, strukturu `UserSession` s Go kanály (`messageChan`, `done`) a logiku multiplexingu (`Publish`) včetně odpojování pomalých klientů. -->
<!-- * **`internal/messaging/sqlite_repository.go`**: Zajišťuje transakční zápis zpráv a správu/mazání doručených zpráv na úrovni SQLite databáze. -->

<!-- = Архітектура та Організація Проекту: Go Messenger -->
<!-- :toc: -->
<!---->
<!-- Проект реалізує розподілену систему обміну повідомленнями (чат), що складається з клієнтського застосунку та двох незалежних серверів (Profile та Messaging). Комунікація між вузлами здійснюється через **gRPC** з використанням двонаправлених потоків (streams) для забезпечення реального часу. -->
<!---->
<!-- == 1. Головні Компоненти  -->
<!---->
<!-- Проект розділено на три незалежні виконувані програми, кожна з яких має власне локальне сховище (SQLite), що гарантує збереження даних після перезавантаження: -->
<!---->
<!-- * `Profile Server`: Відповідає за реєстрацію, автентифікацію, зберігання профілів користувачів (9-значні UserID) та видачу токенів. -->
<!-- * `Messaging Server`: Забезпечує маршрутизацію повідомлень, відстеження стану підключень (Presence) та збереження недоставлених повідомлень (Store-and-Forward). -->
<!-- * `Client (TUI)`: Інтерактивний термінальний клієнт на базі `BubbleTea`. -->
<!---->
<!-- Підтримує локальну історію, роботу з групами, повідомлення про прочитання (acks) та автоматичне перепідключення. -->
<!---->
<!-- == 2. Аутентифікація та Безпека  -->
<!---->
<!-- Для безпечної взаємодії між серверами використовується JWT: -->
<!---->
<!-- * **Зберігання паролів**: Вони хешуються (bcrypt) та безпечно зберігаються виключно на Profile Server. -->
<!-- * **Access та Refresh токени**: При успішному вході Profile Server генерує пару токенів. Access токен (діє 1 годину) використовується для авторизації запитів до Messaging Server. Refresh токен (діє 7 днів) використовується для непомітного оновлення сесії. -->
<!-- * **JTI (JWT ID)**: Сервер профілів відстежує унікальні ідентифікатори Refresh токенів (`activeRefreshTokens`) для запобігання атакам повторного використання (replay attacks). -->
<!-- * **gRPC Interceptors**:  -->
<!--   - На клієнті працює `TokenCredentialsInterecptor`, який автоматично додає токени до заголовків та прозоро оновлює їх при закінченні терміну дії (захищено через `sync.RWMutex` від стану гонки/deadlock-ів). -->
<!--   - На серверах працює `AuthorizationInterceptor`, який перевіряє валідність підпису токенів (через RSA публічні ключі) та додає розпарсені claims у `context`. -->
<!---->
<!-- == 3. Messaging Broker та Горутини -->
<!---->
<!-- Messaging Server використовує внутрішній **Broker** для мультиплексування та доставки повідомлень у реальному часі без використання polling'у: -->
<!---->
<!-- * **Модель Pub/Sub**: Кожен підключений клієнт створює `UserSession`, яка містить буферизований канал (`messageChan`) та сигнальний канал (`done`). -->
<!-- * **Горутини**: Читання з каналу та відправка через gRPC stream відбувається в окремих горутинах. Це дозволяє серверу асинхронно обробляти тисячі підключень. -->
<!-- * **Синхронізація стану (Presence)**: Брокер відстежує активні сесії у thread-safe мапі (`sessionsMu sync.RWMutex`). Якщо користувач має статус "невидимка", сервер продовжує маршрутизацію повідомлень, але віддає клієнтам статус `offline`. -->
<!-- * **Мультиплексування клієнтів**: Один користувач може мати декілька активних клієнтів одночасно. Брокер перебирає всі активні сесії для цільового `UserID` та розсилає повідомлення на всі підключені пристрої користувача. -->
<!-- * **Обробка "повільних" клієнтів**: При переповненні буфера каналу (наприклад, через втрату мережі), сервер безпечно закриває сесію. Повідомлення залишається в базі (SQLite) і буде доставлено як offline-повідомлення при наступному підключенні. -->
<!---->
<!-- == 4. Структура Директорій -->
<!---->
<!-- * `cmd/` - Вхідні точки для компіляції (`client`, `messaging-server`, `profile-server`). -->
<!-- * `internal/auth/` - Логіка JWT, ключі (RSA) та gRPC middleware (interceptors). -->
<!-- * `internal/messaging/` - Логіка чатів: Broker (управління сесіями), сервісний шар та SQLite репозиторій. -->
<!-- * `internal/profiles/` - Бізнес-логіка профілів, валідація, генерація ID та управління статусами. -->
<!-- * `internal/client/` - Логіка клієнта: -->
<!--   - `state/` - Глобальний контекст, конфігурація та підключення. -->
<!--   - `storage/` - Локальна БД клієнта для кешування історії. -->
<!--   - `tui/` - Візуальні компоненти термінального інтерфейсу (моделі, стани екранів). -->
<!-- * `data/` - Файли баз даних SQLite для серверів та індивідуальних клієнтів. -->
<!-- * `protos/` та `generated/` - gRPC контракти (`.proto`) та згенерований Go код. -->
