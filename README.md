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
| **`lybel-docs`** | Assistente da base de conhecimento Confluence. Busca, cria e atualiza páginas no espaço Lybel em linguagem natural. Usa um CLI Go local (`page digest`, `page apply`, `search`) que retorna sub-KB em vez do ADF inteiro — drasticamente mais barato em tokens que o MCP Atlassian puro. MCP fica como fallback. | [SKILL.md](./skills/lybel-docs/SKILL.md) |

---

## Como funciona — modelo bootstrap

A `lybel-docs` é uma skill **timeless**: o repo não guarda dados específicos da Lybel (nomes de advisors, lista de investidores, page IDs de cada parceiro). Em vez disso:

- **Ao usar a skill, o Claude sempre consulta a Home do Confluence primeiro** (pageId `164232`). A Home é a fonte de verdade — mantém taxonomia atual, aliases, status e o índice de page IDs.
- O repo só fornece **estrutura, workflows e templates** — instruções genéricas que não envelhecem.
- **Nenhum dado específico vive no repo** — por isso é safe deixar público no GitHub.
- Os arquivos em `reference/` são apenas **fallback** quando o Confluence está inacessível.

**Para customizar pra outra empresa**: troque `cloudId` e `pageId` da Home no frontmatter e no corpo de [`skills/lybel-docs/SKILL.md`](./skills/lybel-docs/SKILL.md). Crie a Home no seu próprio Confluence seguindo o mesmo padrão (taxonomia + aliases + index).

## Por que CLI em vez de só MCP

O servidor MCP da Atlassian é genérico e devolve o ADF inteiro de cada página (10–40 KB de JSON). Em uma sessão típica de research + edição de docs, isso facilmente queima a janela de contexto. O `lybel-docs` CLI vive no diretório da skill (`~/.claude/skills/lybel-docs/bin/`) e oferece comandos enxutos:

- **`home --refresh`** — uma vez por sessão, baixa a Home do Confluence e guarda em `~/.cache/lybel-docs/home.json` (digest + texto renderizado + ADF parseado). Daí em diante, todo `home --query "termo"` / `--show` / `--digest` é 100% local — zero chamadas pra API.
- **`page digest --page-id ID`** — devolve título, versão, outline de headings, macros e word count em ~500 bytes (vs 10–40 KB do `getConfluencePage`). Resolve a maioria das perguntas "o que tem nessa página?".
- **`page apply --page-id ID --replace-section "X" --fragment file.md`** — GET → edit section-level → PUT atômico, com retry automático em 409 (alguém editou no meio). Suporta também `--table-add-row` / `--table-remove-row` pra atualizar tabelas dentro de seções. O ADF nunca passa pelo contexto do LLM. Macros fora da seção alterada são preservadas byte-a-byte.
- **`search "termo"`** — busca CQL com saída TSV compacta (`pageId\ttitle\turl\texcerpt`).
- **`page get --format export_view`** — quando precisa do conteúdo, devolve o HTML renderizado (~2× menor que ADF).

**Invariante de segurança:** o cache é read-only. Toda escrita (`page apply`, `index add`, etc.) faz GET fresh do ADF antes do PUT — assim você nunca sobrescreve uma alteração feita em outra máquina.

O Claude usa o CLI quando ele existe e cai no MCP automaticamente quando não. Resultado: sessão de docs típica fica 10–50× mais barata em tokens.

---

## Como instalar

Três caminhos, do mais fácil pro mais técnico.

### Opção A — peça pra uma IA instalar pra você (mais fácil)

Se você não está confortável com terminal, abra qualquer agente de IA (Claude, Gemini, ChatGPT, Cursor, etc.) e cole esta mensagem:

> Quero instalar a skill `lybel-docs` no meu computador. A documentação está em https://github.com/lybel-app/skills. Lê o README, identifica meu sistema operacional, roda os comandos de instalação, e me guia pela configuração de credenciais Atlassian no final.

A IA vai ler este README, detectar seu OS, rodar o instalador, e te ajudar a gerar o token Atlassian + configurar tudo. Ver [instruções para agentes de IA](#instruções-para-agentes-de-ia--instalação-assistida) abaixo (a IA usa essa seção como roteiro).

### Opção B — você mesmo no terminal

1. **Instale o Claude Desktop** — baixe em [claude.ai/download](https://claude.ai/download) e faça login com sua conta Lybel.
2. **Rode o instalador:**

   **macOS/Linux:**
   ```bash
   curl -fsSL https://raw.githubusercontent.com/lybel-app/skills/main/skills/lybel-docs/install/install.sh | bash
   ```

   **Windows (PowerShell):**
   ```powershell
   iwr -useb https://raw.githubusercontent.com/lybel-app/skills/main/skills/lybel-docs/install/install.ps1 | iex
   ```

3. **Configure credenciais Atlassian:**
   - Gere um API token em https://id.atlassian.com/manage-profile/security/api-tokens (botão **Create API token**, dá um nome qualquer, copia o valor — só aparece uma vez).
   - Rode: `lybel-docs setup` (ele pergunta email + token interativamente).
   - Valida: `lybel-docs setup --check` deve retornar `credentials valid`.

4. **Reabra o Claude Desktop**, vá em **Code → Settings → Integrations** e conecte sua conta Atlassian via OAuth (necessário pro fallback MCP, caso a CLI falhe).

5. **Pronto.** Pergunte coisas como:
   - *"onde fica a página de parceiros?"*
   - *"cria uma ata de reunião com o Itaú"*
   - *"quais aceleradoras a Lybel está participando?"*

> **Para atualizar:** rode `lybel-docs update`. Pega a última release do GitHub, mantém credenciais e cache.

### Opção C — caminho dev (clone do repo)

```bash
# 1. Clone (cria diretório `skills/`)
git clone https://github.com/lybel-app/skills.git lybel-skills
cd lybel-skills

# 2. Symlink da skill para o diretório do Claude
ln -s "$(pwd)/skills/lybel-docs" ~/.claude/skills/lybel-docs

# 3. (Recomendado) Build do CLI Go — habilita digest/apply/search e reduz custo de tokens
cd skills/lybel-docs/cli && make install
# (Build padrão instala em ~/.claude/skills/lybel-docs/bin/lybel-docs)
# Configurar credenciais Atlassian:
lybel-docs setup
cd -

# 4. Reinicie o Claude Code
#    A skill aparece automaticamente quando você faz uma pergunta relevante.
```

No Windows, troque o `ln -s` por um **diretório junction** (`mklink /J`) ou copie a pasta.

---

## Instruções para agentes de IA — instalação assistida

> Esta seção é um **roteiro pra agentes de IA** (Claude, Gemini, ChatGPT, Cursor, etc.) que estejam ajudando um usuário não-técnico a instalar a skill. Pula esta seção se você for humano — vê a [Opção B](#opção-b--você-mesmo-no-terminal) acima.

### Pré-requisitos
- Usuário tem conta Atlassian (a mesma que usa pra acessar `lybel.atlassian.net`).
- Internet ativa pra baixar do GitHub Releases.

### Passo 1 — Detectar sistema operacional
- macOS / Linux → use `install.sh` (bash).
- Windows → use `install.ps1` (PowerShell 5+).

### Passo 2 — Rodar instalador

**macOS / Linux:**
```bash
curl -fsSL https://raw.githubusercontent.com/lybel-app/skills/main/skills/lybel-docs/install/install.sh | bash
```

**Windows (PowerShell):**
```powershell
iwr -useb https://raw.githubusercontent.com/lybel-app/skills/main/skills/lybel-docs/install/install.ps1 | iex
```

O instalador baixa o último release, descompacta em `~/.claude/skills/lybel-docs/` (ou `%USERPROFILE%\.claude\skills\lybel-docs\` no Windows), e adiciona o binário ao PATH.

### Passo 3 — Configurar credenciais (não interativo)

A skill precisa de **email Atlassian** + **API token**. **Não rode `lybel-docs setup` sem flags em sessão de IA** — ele pede input interativo no stdin e trava.

**3.1 — Guie o usuário a gerar o token:**
1. Diga: "Abra https://id.atlassian.com/manage-profile/security/api-tokens em uma aba nova."
2. "Clique em **Create API token**, dê um nome qualquer (ex: `lybel-docs`)."
3. "Copia o token — atenção: ele só aparece uma vez. Cola aqui."

**3.2 — Pergunte o email** se ainda não souber (provavelmente é o email corporativo do usuário, ex: `nome@lybel.com.br` ou `nome@novapaytech.com`).

**3.3 — Salve as credenciais não-interativamente:**
```bash
lybel-docs setup --email "USER_EMAIL" --token "USER_TOKEN"
```

### Passo 4 — Validar

```bash
lybel-docs setup --check
```

Códigos de saída:
- `0` — credenciais válidas, prossegue.
- `1` — arquivo de credenciais não foi salvo (volta no Passo 3.3).
- `2` — credenciais inválidas (token errado ou revogado — usuário gera novo token).
- `3` — erro de rede (rede do usuário tá ruim, tenta de novo).

### Passo 5 — Smoke test

```bash
lybel-docs home --refresh
```

Deve baixar a Home do Confluence Lybel e cachear localmente. Se imprimir o digest da Home sem erro, instalação completa.

### Troubleshooting comum

| Sintoma | Causa provável | Resolução |
|---|---|---|
| `command not found: lybel-docs` | PATH ainda não atualizado | Pede pro usuário fechar e reabrir o terminal. Alternativa: rode com path absoluto `~/.claude/skills/lybel-docs/bin/lybel-docs ...` |
| `Permission denied` no Linux/macOS | Binário sem flag de execução | `chmod +x ~/.claude/skills/lybel-docs/bin/lybel-docs` |
| Windows SmartScreen bloqueia | Binário sem assinatura | Usuário marca "Run anyway" no aviso |
| `setup --check` retorna 2 | Token revogado ou digitado errado | Gera novo token (Passo 3.1) e refaz Passo 3.3 |
| `setup --check` retorna 3 | Sem internet, ou Atlassian fora | Tentar de novo em alguns minutos |

### O que NÃO fazer
- Não rode `lybel-docs setup` (sem flags) em sessão de IA — é interativo, vai travar.
- Não tente fazer `git clone` do repo pro caminho fácil — isso é a Opção C (dev). O usuário não-técnico não precisa do código fonte.
- Não suba o token do usuário pra nenhum lugar (chat, log, screenshot, repo). Confidencial — o usuário cola só pro `setup` salvar localmente.

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

Cada skill é **self-contained** — tudo relacionado a ela vive em `skills/<nome>/`:

```
lybel-skills/
├── skills/
│   └── <nome-da-skill>/
│       ├── SKILL.md           # Frontmatter + instruções da skill
│       ├── reference/         # Docs auxiliares (templates, taxonomia, workflows…)
│       ├── cli/               # (opcional) Código-fonte do CLI Go que a skill usa
│       │   ├── main.go
│       │   ├── adf/           # pacotes Go
│       │   ├── setup/
│       │   ├── go.mod
│       │   ├── Makefile       # build, test, install
│       │   └── README.md      # docs do CLI
│       ├── install/           # (opcional) Scripts de instalação pra usuário final
│       │   ├── install.sh     # macOS/Linux
│       │   └── install.ps1    # Windows
│       └── bin/               # Binário compilado (gitignored — gerado por make install)
├── .github/workflows/
│   └── release.yml            # Tag v* dispara build cross-platform + GitHub Release
├── LICENSE
└── README.md                  # Este arquivo
```

**Regras da convenção:**
- **Skills sem CLI** simplesmente não têm `cli/`. Funciona normal — `SKILL.md` + `reference/` é o mínimo.
- **Binários** são gerados por `make install` dentro de `skills/<nome>/cli/` — instalam direto em `~/.claude/skills/<nome>/bin/` (caminho de runtime do Claude). O diretório `bin/` no repo é gitignored.
- **Release assets** (ZIPs cross-platform) são gerados pelo CI em runtime, **não vivem no repo**.

### Como adicionar uma skill nova

1. Cria `skills/<nome-da-skill>/SKILL.md` seguindo o formato de [skills.md](https://docs.claude.com/en/docs/claude-code/skills).
2. Adiciona arquivos de referência em `skills/<nome>/reference/`.
3. Se a skill precisa de CLI, cria `skills/<nome>/cli/` com `main.go` + `Makefile`. Se for só prompts/MCP, pula.
4. Testa localmente via symlink (veja [Opção C](#opção-c--caminho-dev-clone-do-repo)).
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
- Windows: `C:\Users\<seu-usuario>\.claude\skills\lybel-docs\`
- macOS/Linux: `~/.claude/skills/lybel-docs/`

**Como desinstalo?**
Apague a pasta acima. Pronto.

**A skill manda meus dados pra algum servidor?**
Não. A skill só roda dentro do seu Claude Desktop/Code e usa a integração Atlassian que você configurou. Nada de telemetria.

---

## License

[MIT](./LICENSE) © 2026 Lybel
