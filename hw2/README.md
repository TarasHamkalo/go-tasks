# Spuštění ukázky
Pro spuštění demonstračního programu použijte:

```bash
go run cmd/demo/main.go
```

V ukázce jsou definované případy použití uvedené na stránce zadání na `courses`.

> **_NOTE:_** Některé příklady zapisují výstup do souborů umístěných v `{project_dir}/logs`.

> **_NOTE:_** Adresář logs se vytvoří při spuštění programu (nikdy se ale automaticky nemaže)

# Adresáře projektu a vybrané soubory

## `logging`
Balík implementující knihovnu pro logování.

### `logging/logger.go`
- Definuje hlavní rozhraní `Logger`.
- Vytváří objekty LogRecord a předává je k serializaci a zápisu do sinků `logging/sink.go`.

### `logging/log_record.go`
- Definuje datovou strukturu `LogRecord`, která reprezentuje jeden logovací záznam
 
### `logging/sink.go`
- Definuje rozhraní `Sink`, které kombinuje formátování a zápis logovacích záznamů.
- Obsahuje implementaci `BaseSink`, která propojuje `Formatter` a `Appender` a filtruje logy podle úrovně.

### `logging/appender.go`
- Definuje rozhraní `Appender`, které je zodpovědné za ukládání/zapisování serializovaných logovacích záznamů.

### `logging/console_appender.go`
- Implementace `Appender`, která zapisuje logy na standardní výstup (`stdout`).

### `logging/file_appender.go`
- Implementace `Appender`, která zapisuje logy do souboru.
- Soubor otevře v režimu `append` nebo ho vytvoří, pokud neexistuje.

### `logging/formatter.go`
- Definuje rozhraní `Formatter`, které zajišťuje serializaci objektu `LogRecord`.
- Obsahuje také ANSI barevné konstanty pro barevný výstup v konzoli.

### `logging/json_formatter.go`
- Implementuje `Formatter`, který serializuje objekty `LogRecord` do formátu `JSON`.

### `logging/stages_formatter.go`
- Poskytuje builder pro `Formatter` složený z více formátovacích kroků.
