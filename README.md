# 🤖 MCP Gorchestrator Server

AI 모델들의 의견을 수집하고 토론하여 최종 결론을 도출하는 오케스트레이터 서버입니다.

## 🎯 개념

```
Claude Code (사용자 질문)
    ↓
Claude가 먼저 답변 생성
    ↓
Orchestrator에 질문 + Claude 의견 전송
    ↓
┌─────────────────┬─────────────────┐
│   Gemini API    │   GPT-4 API     │
│   병렬 호출      │   병렬 호출      │
└────────┬────────┴────────┬────────┘
         │                 │
    Gemini 의견        GPT-4 의견
         │                 │
         └────────┬────────┘
                  ↓
        3개 의견 수집 완료
        (Claude + Gemini + GPT-4)
                  ↓
        🗳️ 투표 시작 (병렬)
                  ↓
    ┌─────────────┴─────────────┐
    │                           │
Gemini 투표              GPT-4 투표
Claude vs GPT-4         Claude vs Gemini
    │                           │
    └─────────────┬─────────────┘
                  ↓
           득표 집계
      Claude: 2표
      Gemini: 0표
      GPT-4: 0표
                  ↓
    ┌─────────────────────────────┐
    │  개별 의견 표시:             │
    │  - 💬 Claude's Opinion      │
    │  - 🔷 Gemini's Opinion      │
    │  - 🟢 GPT-4's Opinion       │
    │                              │
    │  🗳️ 투표 결과:              │
    │  - Gemini's Vote: Claude    │
    │  - GPT-4's Vote: Claude     │
    │                              │
    │  🏆 Winner: Claude (2표)    │
    └─────────────────────────────┘
                  ↓
         Claude Code에 표시
```

## 🚀 설치 및 실행

### 1. 환경 변수 설정

`.env` 파일에 API 키를 설정하세요:

```bash
# Gemini API 키 (Google AI Studio에서 발급)
GEMINI_API_KEY=your_gemini_api_key_here

# OpenAI API 키 (OpenAI Platform에서 발급)
OPENAI_API_KEY=your_openai_api_key_here
```

### 2. 빌드 및 실행

```bash
# 의존성 설치
go mod download

# 빌드
go build

# 실행
./mcp-gorchestrator-server
```

서버가 포트 8080에서 시작됩니다.

## 📡 API 사용법

### 엔드포인트

```
POST http://localhost:8080/orchestrate
```

### 요청 형식

```json
{
  "question": "여기에 질문을 입력하세요",
  "claude_opinion": "Claude의 의견을 여기에 입력하세요"
}
```

### 응답 형식

```json
{
  "response": "# 🤖 AI Model Opinions\n\n## 💬 Claude's Opinion\n...\n\n## 🔷 Gemini's Opinion\n...\n\n## 🟢 GPT-4's Opinion\n...\n\n## 🏆 Final Decision\n..."
}
```

## 🔧 Claude Code MCP 서버로 등록

### 1. Go 서버 실행

먼저 orchestrator 서버를 실행하세요:

```bash
# 프로젝트 디렉토리에서
./mcp-gorchestrator-server

# 또는
go run .
```

서버가 `http://localhost:8080`에서 실행됩니다.

### 2. 프로젝트 빌드

```bash
cd /Users/dd/Dev/mcp-gorchestrator-server
go build
```

이제 실행 파일 `mcp-gorchestrator-server`가 생성됩니다.

### 3. Claude Code에 글로벌 MCP 서버로 추가

`~/.claude.json` 파일을 편집하여 MCP 서버를 추가하세요:

```bash
# 파일 열기
nano ~/.claude.json
# 또는
code ~/.claude.json
```

`mcpServers` 섹션에 다음을 추가:

```json
{
  "mcpServers": {
    "gorchestrator": {
      "type": "stdio",
      "command": "/Users/dd/Dev/mcp-gorchestrator-server/mcp-gorchestrator-server",
      "args": ["mcp"],
      "env": {
        "GEMINI_API_KEY": "your_gemini_api_key_here",
        "OPENAI_API_KEY": "your_openai_api_key_here"
      }
    }
  }
}
```

> **중요**:
> - 경로를 본인의 실제 프로젝트 경로로 변경하세요!
> - 절대 경로를 사용해야 합니다.
> - **API 키를 반드시 입력하세요!** (`.env` 파일의 키 사용)
> - 기존 MCP 서버가 있다면 추가로 입력하세요 (덮어쓰지 마세요)

### 4. Claude Code 재시작

설정을 저장한 후 VSCode를 재시작하거나, Command Palette에서:
```
Claude Code: Reload MCP Servers
```

### 5. 사용 방법

Claude Code에서 다음과 같이 질문하세요:

```
"Go에서 채널과 뮤텍스 중 어떤 것을 사용해야 할까요?
여러 AI 모델의 의견을 들어보고 싶어요."
```

Claude가 자동으로:
1. 먼저 자신의 의견 생성
2. MCP 서버를 통해 Gemini, GPT-4의 의견 수집
3. 투표 진행
4. 종합 결과 표시

### 6. 직접 MCP 도구 호출

또는 명시적으로 MCP 도구를 요청할 수 있습니다:

```
"ask_ai_consensus 도구를 사용해서
'Python vs JavaScript 백엔드 개발' 에 대해 물어봐 줘"
```

### 7. MCP 서버 확인

Claude Code에서 `/mcp` 명령어를 입력하면 등록된 MCP 서버 목록에서 `gorchestrator`를 확인할 수 있습니다.

## 📊 작동 방식

### 병렬 처리
- Gemini와 GPT-4 API를 **동시에** 호출 (goroutine 사용)
- 최대 60초 타임아웃

### 부분 실패 처리
- **3개 의견 모두 성공**: 정상 처리
- **2개 의견 성공**: 2개로 비교 (경고 로깅)
- **1개 의견 성공**: 에러 반환
- **0개 성공**: 에러 반환

### 투표 시스템 (Peer Voting)
각 모델이 다른 모델들을 평가:
1. **Gemini 투표**: Claude vs GPT-4 중 선택 (자기 제외)
2. **GPT-4 투표**: Claude vs Gemini 중 선택 (자기 제외)
3. **득표 집계**: 가장 많은 표를 받은 의견이 승리
4. **동점 처리**: 동점일 경우 모두 우수한 의견으로 표시

> **Note**: Claude는 이미 Claude Code에서 실행 중이므로 투표에 참여하지 않습니다. Gemini와 GPT-4가 상호 평가합니다.

## 🛠️ 프로젝트 구조

```
mcp-gorchestrator-server/
├── main.go           # HTTP 서버 & MCP 서버 엔트리포인트
├── mcp-server.go     # MCP 프로토콜 구현 (stdio)
├── types.go          # 데이터 구조 정의
├── orchestrator.go   # API 호출 & 병렬 처리
├── decision.go       # 투표 시스템 로직
├── go.mod            # Go 모듈 의존성
└── .env              # 환경 변수 (git ignore)
```

### 실행 모드

**MCP 서버 모드 (Claude Code용)**:
```bash
./mcp-gorchestrator-server mcp
```
- stdio를 통해 MCP 프로토콜로 통신
- Claude Code와 직접 통합

**HTTP 서버 모드**:
```bash
./mcp-gorchestrator-server
```
- 포트 8080에서 HTTP API 제공
- 직접 API 호출 가능

## 🔑 필요한 API 키

### Gemini API
- 발급: https://aistudio.google.com/app/api-keys
- 역할: 의견 제시 + 최종 판단

### OpenAI API
- 발급: https://platform.openai.com/api-keys
- 역할: 의견 제시

## 💡 예제

### cURL 테스트

```bash
curl -X POST http://localhost:8080/orchestrate \
  -H "Content-Type: application/json" \
  -d '{
    "question": "Go에서 채널과 뮤텍스 중 어떤 것을 사용해야 하나요?",
    "claude_opinion": "상황에 따라 다릅니다. 데이터를 전달해야 한다면 채널을, 단순히 상태를 보호해야 한다면 뮤텍스를 사용하는 것이 좋습니다."
  }'
```

### 응답 예시

```markdown
# 🤖 AI Model Opinions

## 💬 Claude's Opinion
상황에 따라 다릅니다. 데이터를 전달해야 한다면 채널을, 단순히 상태를 보호해야 한다면 뮤텍스를 사용하는 것이 좋습니다.

---

## 🔷 Gemini's Opinion
Go의 동시성 패턴에서 채널은 "공유 메모리 대신 통신으로 공유하라"는 철학을 따릅니다...

---

## 🟢 GPT-4's Opinion
채널과 뮤텍스는 각각 다른 동시성 문제를 해결합니다...

---

## 🗳️ Voting Results

**Gemini's Vote:** Claude
*Reasoning:* Claude's answer is more concise and practical, directly addressing the core decision criteria.

**GPT-4's Vote:** Claude
*Reasoning:* Claude provides clear, actionable guidance that's easy to apply in real-world scenarios.

### Vote Count:
- 💬 **Claude**: 2 vote(s)
- 🔷 **Gemini**: 0 vote(s)
- 🟢 **GPT-4**: 0 vote(s)

### 🏆 Winner: 💬 **Claude**
Claude's opinion received the most votes from peer models!
```

## 🎨 특징

✅ **AI 집단 지성**: 3개 주요 AI 모델의 의견 수집 (Claude, Gemini, GPT-4)

✅ **공정한 투표 시스템**: 각 모델이 다른 모델들을 평가 (자기 제외)

✅ **병렬 처리**: 의견 수집과 투표를 동시에 진행

✅ **견고한 에러 처리**: 부분 실패 시에도 작동

✅ **구조화된 출력**: 마크다운 형식의 가독성 높은 결과

✅ **투명한 프로세스**: 모든 투표와 근거를 명시

✅ **Claude Code 통합**: MCP 프로토콜 지원


## 📝 라이선스

MIT License

## 🤝 기여

Issues와 Pull Requests를 환영합니다!
