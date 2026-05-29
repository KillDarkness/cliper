# Cliper

Cliper é um aplicativo de clipping para Linux escrito em Go que mantém um buffer contínuo dos últimos 2 minutos da tela e salva um MP4 final quando uma hotkey global é pressionada.

## Arquitetura

O projeto é dividido em quatro módulos principais:

- **Config (`internal/config`)**: centraliza caminhos, duração dos segmentos, quantidade máxima de segmentos, FPS, resolução, hotkey e backend de captura. Também cria automaticamente `buffer/` e `clips/` na inicialização.
- **FFmpeg manager (`internal/ffmpeg`)**: inicia o FFmpeg via `exec.Command`, mantém a gravação contínua em background, grava segmentos MPEG-TS de 5 segundos e reinicia o processo se ele encerrar inesperadamente.
- **Clip saver (`internal/clip`)**: lê a playlist de segmentos finalizados, copia um snapshot dos segmentos atuais para um diretório temporário e concatena tudo com FFmpeg usando `-c copy`, evitando reencodificação e reduzindo uso de CPU.
- **Hotkey manager (`internal/hotkey`)**: registra uma hotkey global no X11 usando XGrabKey. No Wayland, hotkeys globais são restritas por design; nesse caso o app mantém um fallback por terminal, pressionando Enter para salvar um clip.

## Fluxo de funcionamento

1. Na inicialização, `buffer/` e `clips/` são criados automaticamente.
2. O FFmpeg começa a capturar a tela e gera segmentos `.ts` de 5 segundos em `buffer/`.
3. A playlist `buffer/segments.ffconcat` mantém somente os últimos 24 segmentos finalizados, totalizando cerca de 2 minutos.
4. Ao pressionar a hotkey, o clip saver copia os segmentos listados para um snapshot temporário.
5. O FFmpeg concatena o snapshot com `-c copy` e salva `clips/clip_TIMESTAMP.mp4`.

A cópia para snapshot evita que segmentos sejam removidos ou alterados durante a concatenação.

## Requisitos no Arch Linux

```bash
sudo pacman -S ffmpeg go libx11
```

Para X11, o backend padrão usa `x11grab`. Em Wayland, use `pipewire` se o seu FFmpeg tiver suporte ao input PipeWire, ou `kmsgrab` em cenários avançados com permissões adequadas.

## Uso

```bash
go run ./cmd/cliper
```

Por padrão, a hotkey é `F8` no X11. Variáveis de ambiente úteis:

- `CLIPER_BACKEND`: `x11`, `pipewire` ou `kmsgrab`.
- `CLIPER_DISPLAY`: display X11, padrão `$DISPLAY` ou `:0.0`.
- `CLIPER_VIDEO_SIZE`: resolução, padrão `1920x1080`.
- `CLIPER_FPS`: FPS, padrão `60`.
- `CLIPER_HOTKEY`: `F8`, `F9`, `F10`, `F11` ou `F12`.
- `CLIPER_FFMPEG`: caminho do binário FFmpeg, padrão `ffmpeg`.
- `CLIPER_BUFFER_DIR`: diretório de buffer, padrão `buffer`.
- `CLIPER_CLIPS_DIR`: diretório dos clips, padrão `clips`.

Exemplos:

```bash
CLIPER_BACKEND=x11 CLIPER_DISPLAY=:0.0 go run ./cmd/cliper
CLIPER_BACKEND=pipewire go run ./cmd/cliper
CLIPER_BACKEND=kmsgrab go run ./cmd/cliper
```

## Observações sobre Wayland

Wayland não permite hotkeys globais arbitrárias para clientes comuns. Para uso completo em Wayland, a abordagem recomendada é integrar com atalhos do compositor chamando uma interface externa do app. Esta versão mantém suporte experimental a captura PipeWire/KMSGrab, mas a hotkey global nativa é implementada para X11.
