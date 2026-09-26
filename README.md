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

## Desarrollo

```bash
go test ./...
CINEXPLORER_PROBE_CORPUS=D:/cine go test ./internal/probe -run Corpus -v -timeout 0
go run ./cmd/cinexplorer -dir .run
bash scripts/build.sh
```
