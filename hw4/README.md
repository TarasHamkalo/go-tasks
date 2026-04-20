## HTTP Mocker
Projekt implementuje HTTP mock server,
který na základě definovaných pravidel generuje odpovědi na příchozí HTTP
požadavky. Server podporuje současnou obsluhu na protokolech
HTTP i HTTPS a umožňuje konfiguraci za běhu pomocí gRPC rozhraní.

### Architektura a organizace projektu
Vybrané balíčky projektu:
* **`cmd/app`**
    * Vstupní bod aplikace.
    * Obsahuje inicializaci logování (Zap), načítání certifikátů a spuštění HTTP/S a gRPC serverů.
* **`pkg/mocker`**
    * Jádro mockeru.
    * Obsahuje logiku pro vyhledávání (matching) a správu konfigurací.
    * Je nezávislý na transportní vrstvě.
* **`internal/handler`**
    * Implementace HTTP a gRPC API.
* **`internal/middleware`**
    * Implementace middlewaru pro logování požadavků a trasování pomocí ID požadavku (Trace ID).
* **`internal/server`**
    * Implementace wrapperů pro HTTP/S a gRPC servery.
    * Zajišťují jejich spuštění a korektní ukončení (graceful shutdown).

Toto rozdělení umožňuje použít logiku mockeru nezávisle na transportní vrstvě.

#### Důležité adresáře projektu

* **`certs`**: Obsahuje serverové certifikáty pro demo účely (HTTPS).
* **`protos`**: Zdrojové soubory definující gRPC rozhraní.
* **`generated`**: Vygenerovaný kód (stubs) pro gRPC.

---

### Použití mockeru jako knihovny

Jádro mockeru lze využít i jako samostatnou knihovnu. Níže je ukázka programové konfigurace cest a následné obsluhy požadavku:

```go
import "http-mocker/pkg/mocker"

func setTestRoutes(m *mocker.HttpMocker) {
    m.SetReply(
        mocker.NewRequestSpec(
            "/users", 
            "GET", 
            nil,   // no query parameters
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

## Spuštění a ověření

Pro spuštění aplikace použijte příkaz:

```bash
go run cmd/app/main.go
```


**_NOTE:_**
Porty 8081 (gRPC, TLS), 8080 (HTTP) a 8443 (HTTPS) musí být dostpuné


Pro otestování základních funkcí je přiložen skript `demo/demo.sh`, 
který využívá nástroje `grpcurl` a `curl` k ověření požadovaných scénářů.

### Ukázka běhu
Výstupy aplikace po spuštění skriptu `demo/demo.sh` naleznete v adresáři `demo`:
- `demo/demo-sh-mocker.log`: log aplikace ve formátu JSON
- `demo/demo-sh-mocker-console.log`: výstup aplikace na stdout
- `demo/demo-sh-output.txt`: kompletní výstup testovacího skriptu `demo/demo.sh`

