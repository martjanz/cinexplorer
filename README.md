# Cinexplorer

Explorador local de una colección de películas. Lee las carpetas configuradas
sin modificarlas y guarda su catálogo junto al ejecutable.

## Uso

1. Copiá la carpeta `cinexplorer/` (con los tres ejecutables) a la raíz del disco,
   al lado de tus carpetas de películas.
2. Ejecutá el binario de tu sistema:
   - Windows: `cinexplorer-windows.exe`
   - macOS: `cinexplorer-macos` (la primera vez: clic derecho → Abrir, porque no está firmado)
   - Linux: `./cinexplorer-linux`
3. Se abre el navegador. En el primer uso se crea `config.json` con las carpetas
   hermanas como raíces; editalo para cambiarlas.

Si la carpeta no se puede escribir (por ejemplo, un disco NTFS en macOS), la app
arranca en modo consulta con el catálogo existente.

Además de los nombres, la app lee los encabezados de los videos (MKV, MP4/MOV,
AVI e IFO de DVD) para conocer resolución, codecs, duración y pistas de audio y
subtítulos. Si `ffprobe` está instalado y en el PATH, se usa para los formatos
que no lee por su cuenta (RMVB, MPG, WMV…); no es obligatorio.

Para identificar las películas se usa TMDB (y Wikidata para completar datos).
Pegá tu token de lectura de TMDB (API Read Access Token, v4) en `config.json`:

```json
{
  "roots": ["../cine", "../cine-ordenar"],
  "tmdbToken": "eyJ…",
  "language": "es-ES",
  "imagePrefetch": "none"
}
```

Sin red, la identificación se reintenta sola. Las que no se pueden decidir
aparecen en la pestaña "Sin identificar" con candidatos para confirmar.
`imagePrefetch` baja imágenes por adelantado: `"posters"` (afiches), `"all"`
(afiches y escenas) o `"none"` (solo las que se van mirando).

## Desarrollo

```bash
go test ./...
CINEXPLORER_PROBE_CORPUS=D:/cine go test ./internal/probe -run Corpus -v -timeout 0
CINEXPLORER_TMDB_RECORD=1 go test ./internal/identify -run Corpus -v
go run ./cmd/cinexplorer -dir .run
bash scripts/build.sh
```
