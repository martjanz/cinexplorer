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

## Desarrollo

```bash
go test ./...
go run ./cmd/cinexplorer -dir .run
bash scripts/build.sh
```
