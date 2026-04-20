## HTTP Mocker
Projekt implementuje HTTP mock server, který na základě definovaných pravidel 
generuje odpovědi na příchozí HTTP požadavky.
Server podporuje současnu obsluhu na protokolech HTTP i HTTPS a umožňuje konfiguraci za běhu pomocí 
gRPC rozhraní.

### Architektura a organizace projektu
Výbrane balíčky projektu:
- `cmd/app`  
   - Vstupní bod aplikace.
   - Obsahuje inicializaci logování (Zap), načítání certifikátů a spuštění HTTP/S a gRPC serverů.
- `pkg/mocker`
  - Jádro mockru.
  - Obsahuje logiku pro vyhledávání a správu konfigurací.
  - Je nezávislý na transportní vrstvě.
- `internal/handler`
  - Implementace HTTP a gRPC API
- `interal/middleware`
  - Implementace middleweru pro logování požadavků a trasování pomocí ID požadavku.
- `internal/server`
  - Implementace wrapperů pro HTTP/S a gRPC servery.
  - Zajišťují jejich spuštění a korektní ukončení (graceful shutdown).


Toto rozdeleni umoznuje pouzit mocker nezavisle na transport logice.

Dulezite adresare projektů
- `certs`
  - pro demo účely jsou do něho přídané certifikaty servrů
- `protos`
  - Zdrojové soubory definující gRPC rozhraní
- `generated`
  - Vygenerovaný kód (stubs) pro gRPC

## Použití mockru jako knihovny
Jadro mockru muzete vyuzit jako knihovnů, nize je uvedeno nastaveni cest a handlovani
```go
import "http-mocker/pkg/mocker"

func setTestRoutes(m *mocker.HttpMocker) {
    m.SetReply(
        mocker.NewRequestSpec(
            "/users", 
            "GET", 
            nil,   // no query parametry
            false, // no body
            nil,
        ),
        mocker.NewResponseSpec(
            200, 
            map[string][]string{"Content-Type": {"application/json"}}, 
            true, 
            []byte(`["user-1","user-2"]`),
        ),
    )

    m.SetReply(
        mocker.NewRequestSpec(
            "/test", 
            "POST", 
            nil, 
            true, 
            []byte("aaa"),
        ),
        mocker.NewResponseSpec(
            200, 
            nil, 
            true, 
            []byte("ok"),
        ),
    )
}

func handle(r *http.Request, m *mocker.HttpMocker) {
// parse request to mocker.RequestSpec
responseSpec, err := m.Serve(requestSpec)
// write response
}
```

## Spusteni a overeni

Pro spuštění programu použijte:

```bash
go run cmd/app/main.go
```


IMPORTANT: porty 8081 (gRPC, tls), 8080 (HTTP), 8443 (HTTPS) musi byt volne.

Pro otestovani je pridan skript `demo/demo.sh`  ktere pouziva `grpcurl` a `curl` pro otestovani
požadovanych funkci.

### Ukázka běhu
Výstup aplikace po sputeni skriptu `demo/demo.sh` je pridan do adresaru `demo`:
- `demo/demo-sh-mocker.log`: log aplikace v jsonc formatu
- `demo/demo-sh-mocker-console.log`: vystup aplikace na stdout
- `demo/demo-sh-output.txt`: vystup skriptu `demo/demo.sh`

