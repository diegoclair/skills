# Lybel Skills

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)
[![Latest Release](https://img.shields.io/github/v/release/lybel-app/skills?color=11C47E&label=release)](https://github.com/lybel-app/skills/releases/latest)
[![Claude Skills](https://img.shields.io/badge/Claude-Skills-11C47E)](https://docs.claude.com/en/docs/claude-code/skills)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-11C47E)](https://github.com/lybel-app/skills/pulls)

> Skills do Claude para o time da **Lybel**. Pergunte em português; o Claude vai direto no Confluence, Jira e afins.

---

## O que é

`lybel-skills` é um repositório público que reúne **Claude Skills** prontas para o time da Lybel. Uma *skill* é um pacote de instruções + arquivos de referência que o Claude carrega sob demanda quando você faz uma pergunta que bate com o escopo dela.

Hoje só temos uma skill, mas o repo foi desenhado para crescer: próximas candidatas incluem `lybel-jira`, `lybel-figma`, `lybel-analytics`, etc.

## Skills disponíveis

| Skill | Resumo | Docs |
|---|---|---|
| **`confluence-docs`** | Assistente da base de conhecimento Confluence. Busca, cria e atualiza páginas no espaço Lybel em linguagem natural. Usa um CLI Go local (`page digest`, `page apply`, `search`) que retorna sub-KB em vez do ADF inteiro — drasticamente mais barato em tokens que o MCP Atlassian puro. MCP fica como fallback. | [SKILL.md](./confluence-docs/SKILL.md) |

---

## Como funciona — modelo bootstrap

A `confluence-docs` é uma skill **timeless**: o repo não guarda dados específicos da Lybel (nomes de advisors, lista de investidores, page IDs de cada parceiro). Em vez disso:

- **Ao usar a skill, o Claude sempre consulta a Home do Confluence primeiro** (pageId `164232`). A Home é a fonte de verdade — mantém taxonomia atual, aliases, status e o índice de page IDs.
- O repo só fornece **estrutura, workflows e templates** — instruções genéricas que não envelhecem.
- **Nenhum dado específico vive no repo** — por isso é safe deixar público no GitHub.
- Os arquivos em `reference/` são apenas **fallback** quando o Confluence está inacessível.

**Para customizar pra outra empresa**: troque `cloudId` e `pageId` da Home no frontmatter e no corpo de [`confluence-docs/SKILL.md`](./confluence-docs/SKILL.md). Crie a Home no seu próprio Confluence seguindo o mesmo padrão (taxonomia + aliases + index).

## Por que CLI em vez de só MCP

O servidor MCP da Atlassian é genérico e devolve o ADF inteiro de cada página (10–40 KB de JSON). Em uma sessão típica de research + edição de docs, isso facilmente queima a janela de contexto. O `confluence-docs` CLI vive no diretório da skill (`~/.claude/skills/confluence-docs/bin/`) e oferece comandos enxutos:

- **`home --refresh`** — uma vez por sessão, baixa a Home do Confluence e guarda em `~/.cache/confluence-docs/home.json` (digest + texto renderizado + ADF parseado). Daí em diante, todo `home --query "termo"` / `--show` / `--digest` é 100% local — zero chamadas pra API.
- **`page digest --page-id ID`** — devolve título, versão, outline de headings, macros e word count em ~500 bytes (vs 10–40 KB do `getConfluencePage`). Resolve a maioria das perguntas "o que tem nessa página?".
- **`page apply --page-id ID --replace-section "X" --fragment file.md`** — GET → edit section-level → PUT atômico, com retry automático em 409 (alguém editou no meio). Suporta também `--table-add-row` / `--table-remove-row` pra atualizar tabelas dentro de seções. O ADF nunca passa pelo contexto do LLM. Macros fora da seção alterada são preservadas byte-a-byte.
- **`search "termo"`** — busca CQL com saída TSV compacta (`pageId\ttitle\turl\texcerpt`).
- **`page get --format export_view`** — quando precisa do conteúdo, devolve o HTML renderizado (~2× menor que ADF).

**Invariante de segurança:** o cache é read-only. Toda escrita (`page apply`, `index add`, etc.) faz GET fresh do ADF antes do PUT — assim você nunca sobrescreve uma alteração feita em outra máquina.

O Claude usa o CLI quando ele existe e cai no MCP automaticamente quando não. Resultado: sessão de docs típica fica 10–50× mais barata em tokens.

---

## Como instalar

> **Resumo:** rode o instalador, configure credenciais, pronto.

### 1. Rode o instalador

**macOS / Linux:**
```bash
curl -fsSL https://raw.githubusercontent.com/lybel-app/skills/main/confluence-docs/install/install.sh | bash
```

**Windows (PowerShell):**
```powershell
iwr -useb https://raw.githubusercontent.com/lybel-app/skills/main/confluence-docs/install/install.ps1 | iex
```

O script:
- Baixa o último release pra sua plataforma e instala em `~/.claude/skills/confluence-docs/` (Linux/macOS) ou `%USERPROFILE%\.claude\skills\confluence-docs\` (Windows).
- Coloca o binário no PATH automaticamente — adiciona uma linha no seu `~/.zshrc` / `~/.bashrc` / `~/.profile` (ou no User PATH no Windows). Idempotente — re-rodar não duplica.
- Imprime no final se as credenciais já estão configuradas.

Se o instalador alterou seu shell profile, **abra um terminal novo** (ou rode `source ~/.zshrc`) pra `confluence-docs` aparecer no PATH da sessão atual.

### 2. Gere um token Atlassian

1. Acesse https://id.atlassian.com/manage-profile/security/api-tokens.
2. Clique em **Create API token**, dá um nome qualquer (ex: `confluence-docs`).
3. **Copie o token na hora** — ele só aparece uma vez.

### 3. Salve as credenciais

```bash
confluence-docs setup
```

O wizard pede seu email Atlassian e o token, valida com a API, e grava em `~/.config/confluence-docs/credentials` (ou `%APPDATA%\confluence-docs\credentials` no Windows) com permissão `0600`.

Se preferir não interativo (CI, automação, ou agente de IA):

```bash
confluence-docs setup --email "seu@email.com" --token "ATATT3xFf..."
```

Valida no final:

```bash
confluence-docs setup --check
# deve imprimir: credentials valid (Seu Nome)
```

### 4. Use

Reabra o Claude Code (ou Claude Desktop) e pergunte coisas como:

- *"onde fica a página de parceiros?"*
- *"cria uma ata de reunião com o Itaú"*
- *"quais aceleradoras a Lybel está participando?"*

A skill aparece automaticamente quando a pergunta bate com o escopo dela.

> **Atualizar:** rode `confluence-docs update`. Pega a última release do GitHub, mantém credenciais e cache intactos.
>
> **Desinstalar:** apague `~/.claude/skills/confluence-docs/` e `~/.config/confluence-docs/` (Linux/macOS) ou as pastas equivalentes no Windows. Remover a linha que o instalador adicionou no seu shell profile é opcional.

---

## Instalação assistida por IA

Não está confortável com terminal? Abra qualquer agente de IA (Claude, Gemini, ChatGPT, Cursor, …) e cole:

> Quero instalar a skill `confluence-docs`. Segue o roteiro em https://github.com/lybel-app/skills/blob/main/confluence-docs/INSTALL_FOR_AI.md — detecta meu sistema operacional, roda os comandos, e me guia pela geração do token Atlassian no final.

O arquivo [`INSTALL_FOR_AI.md`](./confluence-docs/INSTALL_FOR_AI.md) é um runbook pensado pra agentes — exit codes determinísticos, regras de segurança pro token, e troubleshooting passo a passo. Você não precisa lê-lo, só passar a URL pro agente.

---

## Instalação dev (contribuir com a skill)

> Esse caminho é só pra quem vai **modificar** a skill. Se você só quer usar, fica na seção [Como instalar](#como-instalar) acima.

```bash
git clone https://github.com/lybel-app/skills.git
cd skills/confluence-docs/cli

# Build + install do binário no diretório do Claude.
make install
# Instala em ~/.claude/skills/confluence-docs/bin/confluence-docs por padrão.
# Override: make install INSTALL_DIR=/custom/path

# Copie SKILL.md + reference/ pro mesmo diretório (o instalador faz isso
# automaticamente; manualmente:)
mkdir -p ~/.claude/skills/confluence-docs/reference
cp ../SKILL.md ~/.claude/skills/confluence-docs/SKILL.md
cp ../reference/*.md ~/.claude/skills/confluence-docs/reference/

# Configure credenciais e valida.
~/.claude/skills/confluence-docs/bin/confluence-docs setup
~/.claude/skills/confluence-docs/bin/confluence-docs setup --check
```

Se preferir desenvolver editando os arquivos no clone (sem precisar copiar a cada mudança), use um **symlink** apontando o clone para o diretório do Claude:

```bash
mkdir -p ~/.claude/skills
ln -s "$(pwd)/.." ~/.claude/skills/confluence-docs
```

(No Windows, use `mklink /J` em vez de `ln -s`.) Cuidado: alguns ambientes sandbox de IA bloqueiam symlinks dentro de `~/.claude/skills/`. Se for o caso, copie em vez de symlinkar.

---

## Uso típico

### 1. "Onde fica a página X?"

```
Você: onde fica a página de governança?

Claude: Achei no Confluence:
- Governança Lybel — estrutura de comitês, cadência de board meetings e RACI
  https://lybel.atlassian.net/wiki/spaces/lybel/pages/229891
```

### 2. Criar página nova com template

```
Você: cria uma página de parceiro novo: Banco Fictício

Claude: Vou criar em Parceiros → Bancos usando o template "Ficha de Parceiro".
        Título sugerido: "Banco Fictício — Ficha de Parceiro"
        Parent: Parceiros / Bancos (pageId 164500)
        Confirma?

Você: sim

Claude: Criado: https://lybel.atlassian.net/wiki/spaces/lybel/pages/998877
```

### 3. Listar páginas de uma categoria

```
Você: quais aceleradoras a gente tem mapeadas?

Claude: No espaço Lybel, categoria Aceleração:
- Cubo Itaú — 2026 cohort (Aceleração/Programas)
- Distrito Fintech — em avaliação
- Endeavor — contato preliminar
...
```

### 4. Atualizar página existente

```
Você: adiciona na página do Cubo Itaú que a reunião de kickoff ficou pra 05/05

Claude: Encontrei a página "Cubo Itaú — 2026 cohort" (version 7).
        Vou adicionar na seção "Cronograma" a linha:
        "05/05 — Kickoff com mentores"
        Confirma?
```

### 5. Status de algo

```
Você: qual o status da parceria com Nubank?

Claude: Página "Nubank — Parceria" (última atualização: 2026-04-12):
        Label: "status:negociacao"
        Responsável: Diego Clair
        Próximo passo: enviar proposta revisada até 25/04
```

---

## Desenvolvendo

### Estrutura do repo (convenção)

Cada skill é **self-contained** — vive numa pasta no root do repo, com nome igual ao da skill:

```
lybel-skills/
├── <nome-da-skill>/
│   ├── SKILL.md           # Frontmatter + instruções da skill
│   ├── reference/         # Docs auxiliares (templates, taxonomia, workflows…)
│   ├── cli/               # (opcional) Código-fonte do CLI Go que a skill usa
│   │   ├── main.go
│   │   ├── adf/           # pacotes Go
│   │   ├── setup/
│   │   ├── go.mod
│   │   ├── Makefile       # build, test, install
│   │   └── README.md      # docs do CLI
│   ├── install/           # (opcional) Scripts de instalação pra usuário final
│   │   ├── install.sh     # macOS/Linux
│   │   └── install.ps1    # Windows
│   └── bin/               # Binário compilado (gitignored — gerado por make install)
├── .github/workflows/
│   └── release.yml        # Tag v* dispara build cross-platform + GitHub Release
├── LICENSE
└── README.md              # Este arquivo
```

**Regras da convenção:**
- **Skills sem CLI** simplesmente não têm `cli/`. Funciona normal — `SKILL.md` + `reference/` é o mínimo.
- **Binários** são gerados por `make install` dentro de `<nome>/cli/` — instalam direto em `~/.claude/skills/<nome>/bin/` (caminho de runtime do Claude). O diretório `bin/` no repo é gitignored.
- **Release assets** (ZIPs cross-platform) são gerados pelo CI em runtime, **não vivem no repo**.

### Como adicionar uma skill nova

1. Cria `<nome-da-skill>/SKILL.md` seguindo o formato de [skills.md](https://docs.claude.com/en/docs/claude-code/skills).
2. Adiciona arquivos de referência em `<nome>/reference/`.
3. Se a skill precisa de CLI, cria `<nome>/cli/` com `main.go` + `Makefile`. Se for só prompts/MCP, pula.
4. Testa localmente via symlink (veja [Instalação dev](#instalação-dev-contribuir-com-a-skill)).
5. Abre PR. Após merge, criar tag `vX.Y.Z` dispara o workflow de release que monta os ZIPs cross-platform e publica no GitHub Releases automaticamente.

### Convenções

- `name` no frontmatter: lowercase com hífens, máx 64 chars.
- `description`: máx 1024 chars, inclua triggers (frases que ativam a skill).
- Corpo do SKILL.md em **português (Brasil)** — frontmatter fica em inglês.
- Referências usam paths relativos: `reference/foo.md`, **não** URLs absolutos.

### Rodar testes (quando existirem)

```bash
make test          # testa o CLI Go
make lint          # golangci-lint
```

---

## FAQ

**Preciso ter conta GitHub?**
Não. O repo é público e o ZIP do instalador também. Só baixar e rodar.

**Preciso ter git instalado?**
Não, se você usar o instalador (caminho fácil). Só precisa de git se for desenvolvedor e quiser clonar.

**E se eu quiser atualizar a skill?**
Re-execute `install.bat` ou `install.sh`. O instalador é idempotente — ele apaga a versão anterior e instala a nova.

**Funciona em Linux?**
Sim. O `install.sh` detecta macOS e Linux automaticamente. Requer `curl` (ou `wget`) e `unzip`.

**O instalador precisa de permissão de administrador?**
Não. A skill é instalada no seu diretório de usuário (`~/.claude/skills/` ou `%USERPROFILE%\.claude\skills\`).

**Onde os arquivos ficam no meu computador?**
- Windows: `C:\Users\<seu-usuario>\.claude\skills\confluence-docs\`
- macOS/Linux: `~/.claude/skills/confluence-docs/`

**Como desinstalo?**
Apague a pasta acima. Pronto.

**A skill manda meus dados pra algum servidor?**
Não. A skill só roda dentro do seu Claude Desktop/Code e usa a integração Atlassian que você configurou. Nada de telemetria.

---

## License

[MIT](./LICENSE) © 2026 Lybel
