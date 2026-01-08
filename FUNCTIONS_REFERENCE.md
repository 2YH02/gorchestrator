# MCP Gorchestrator - Functions & Features Reference

## 📋 목차
- [개요](#개요)
- [MCP 도구](#mcp-도구)
- [주요 함수](#주요-함수)
- [데이터 타입](#데이터-타입)
- [실행 모드](#실행-모드)
- [워크플로우](#워크플로우)

---

## 개요

MCP Gorchestrator는 여러 AI 모델(Claude, Gemini, GPT-4)의 의견을 수집하고 민주적 투표를 통해 최종 결정을 내리는 MCP(Model Context Protocol) 서버입니다.

**주요 특징:**
- 3개 AI 모델의 동시 의견 수집
- 피어 투표 시스템을 통한 의견 평가
- MCP stdio 모드 및 HTTP 서버 모드 지원

---

## MCP 도구

### `ask_ai_consensus`

**설명:** 여러 AI 모델에게 질문하고 민주적 투표를 통해 최선의 답변을 결정합니다.

**파라미터:**
- `question` (string, required): 모든 AI 모델에게 물어볼 질문
- `claude_opinion` (string, required): Claude의 초기 의견 (다른 모델들과 공유됨)

**반환값:**
- 모든 모델의 의견
- 투표 결과
- 최종 우승자 또는 동점 정보

**사용 예시:**
```json
{
  "question": "이 코드를 리뷰해주세요",
  "claude_opinion": "이 코드는 좋지만 에러 처리가 부족합니다..."
}
```

---

## 주요 함수

### 📁 main.go

#### `main()`
서버의 진입점. 실행 모드를 결정합니다.

**기능:**
- 환경 변수 로드 (.env 파일)
- MCP stdio 모드 또는 HTTP 서버 모드 선택
- 기본: HTTP 서버 (포트 8080)
- `mcp` 인자로 실행 시: stdio 모드

#### `mcpHandler(w http.ResponseWriter, r *http.Request)`
HTTP 엔드포인트 `/orchestrate`의 핸들러

**기능:**
- POST 요청만 허용
- MCPRequest JSON 파싱
- RunConsensusWorkflow 호출
- MCPResponse JSON 반환

---

### 📁 mcp-server.go

#### `RunMCPServer()`
MCP JSON-RPC 프로토콜을 통한 stdio 서버 실행

**기능:**
- stdin에서 JSON-RPC 요청 읽기
- stdout으로 JSON-RPC 응답 전송
- 요청 타입별 핸들러 라우팅

#### `handleInitialize(writer *bufio.Writer, req *JSONRPCRequest)`
MCP 초기화 요청 처리

**반환:**
- Protocol Version: "2024-11-05"
- Server Name: "gorchestrator"
- Server Version: "1.0.0"
- Capabilities: tools 지원

#### `handleToolsList(writer *bufio.Writer, req *JSONRPCRequest)`
사용 가능한 도구 목록 반환

**반환:** `ask_ai_consensus` 도구 정의

#### `handleToolCall(writer *bufio.Writer, req *JSONRPCRequest)`
도구 호출 실행

**기능:**
- 파라미터 검증 (question, claude_opinion)
- RunConsensusWorkflow 호출
- 결과를 ToolResult 형식으로 반환

---

### 📁 orchestrator.go

#### `CallGeminiAPI(ctx context.Context, question string) (string, error)`
Gemini API 직접 호출

**기능:**
- GEMINI_API_KEY 환경 변수 확인
- genai 클라이언트 생성
- gemini-2.0-flash-exp 모델 사용
- 빈 응답 체크
- 디버그 로그 출력

**에러 처리:**
- API 키 미설정
- 클라이언트 생성 실패
- 컨텐츠 생성 실패
- 빈 응답

#### `CallGPT4API(ctx context.Context, question string) (string, error)`
OpenAI GPT-4 API 직접 호출

**기능:**
- OPENAI_API_KEY 환경 변수 확인
- gpt-4o-mini 모델 사용
- HTTP POST 요청 (https://api.openai.com/v1/chat/completions)
- 60초 타임아웃

**에러 처리:**
- API 키 미설정
- 네트워크 오류
- 비정상 상태 코드
- 응답 파싱 실패

#### `RunConsensusWorkflow(request *MCPRequest) (string, error)`
전체 합의 워크플로우 실행

**프로세스:**
1. 90초 타임아웃 컨텍스트 생성
2. Gemini, GPT-4 병렬 호출 (goroutine)
3. 의견 수집 및 에러 처리
4. 최소 2개 의견 확보 확인
5. 3개 모두 있으면 투표 진행
6. 2개만 있으면 간단한 비교
7. FormatOpinions로 결과 포맷팅

**반환:** 모든 의견과 최종 결정을 포함한 마크다운 텍스트

#### `FormatOpinions(opinions *ModelOpinions, finalDecision string) string`
의견들을 읽기 쉬운 마크다운으로 포맷팅

**출력 구조:**
```
# 🤖 AI Model Opinions

## 💬 Claude's Opinion
...

## 🔷 Gemini's Opinion
...

## 🟢 GPT-4's Opinion
...

## 🏆 Final Decision
...
```

---

### 📁 decision.go

#### `CallModelForVote(ctx context.Context, voter string, opinions *ModelOpinions, excludeSelf string) (*VoteResult, error)`
특정 모델에게 다른 모델들의 의견 평가 요청

**파라미터:**
- `voter`: 투표하는 모델 ("Gemini", "GPT-4")
- `opinions`: 모든 모델의 의견
- `excludeSelf`: 투표 대상에서 제외할 모델

**기능:**
- 투표 후보 선정 (자기 자신과 excludeSelf 제외)
- 평가 프롬프트 생성
- JSON 응답 파싱
- VoteResult 반환

**평가 기준:**
- 정확성과 정밀도
- 완성도와 깊이
- 명확성과 구성
- 실용적 유용성

#### `callGeminiForEvaluation(ctx context.Context, prompt string) (string, error)`
투표를 위한 Gemini API 호출

**개선사항:**
- ✅ 빈 응답 체크
- ✅ 디버그 로깅
- ✅ 에러 래핑

#### `callGPT4ForEvaluation(ctx context.Context, prompt string) (string, error)`
투표를 위한 GPT-4 API 호출

**기능:** CallGPT4API 재사용

#### `ConductVoting(opinions *ModelOpinions) (*VotingSummary, error)`
피어 투표 프로세스 실행

**프로세스:**
1. Gemini가 Claude vs GPT-4 평가
2. GPT-4가 Claude vs Gemini 평가
3. (Claude는 이미 실행 중이므로 투표 불가)
4. 투표 수집 및 집계
5. 우승자 결정 또는 동점 확인

**반환:** VotingSummary (투표 결과, 득표수, 우승자)

#### `DetermineFinalSolution(opinions *ModelOpinions) (string, error)`
투표를 통해 최종 솔루션 결정

**기능:**
- ConductVoting 호출
- 투표 결과 포맷팅
- 개별 투표와 이유 표시
- 득표수 표시
- 우승자 또는 동점 결과 표시

---

## 데이터 타입

### 📁 types.go

#### `MCPRequest`
MCP 엔드포인트 수신 요청 구조

```go
type MCPRequest struct {
    Question      string `json:"question"`       // 질문
    ClaudeOpinion string `json:"claude_opinion"` // Claude 의견
}
```

#### `MCPResponse`
MCP 엔드포인트 응답 구조

```go
type MCPResponse struct {
    Response string `json:"response"` // 최종 응답 (마크다운)
}
```

#### `ModelOpinions`
모든 모델의 의견 저장

```go
type ModelOpinions struct {
    Claude      string `json:"claude"`                    // Claude 의견
    Gemini      string `json:"gemini"`                    // Gemini 의견
    GPT4        string `json:"gpt4"`                      // GPT-4 의견
    GeminiError string `json:"gemini_error,omitempty"`    // Gemini 에러
    GPT4Error   string `json:"gpt4_error,omitempty"`      // GPT-4 에러
}
```

#### `VoteResult`
개별 투표 결과

```go
type VoteResult struct {
    Voter     string `json:"voter"`      // 투표자
    ChosenOne string `json:"chosen_one"` // 선택된 모델
    Reasoning string `json:"reasoning"`  // 선택 이유
}
```

#### `VotingSummary`
최종 투표 요약

```go
type VotingSummary struct {
    Votes      []VoteResult   `json:"votes"`       // 모든 투표
    VoteCounts map[string]int `json:"vote_counts"` // 득표수
    Winner     string         `json:"winner"`      // 우승자
    IsTie      bool           `json:"is_tie"`      // 동점 여부
    TiedModels []string       `json:"tied_models"` // 동점 모델들
}
```

#### `EvaluationResponse`
모델의 평가 응답 (JSON)

```go
type EvaluationResponse struct {
    ChosenModel string `json:"chosen_model"` // 선택된 모델
    Reasoning   string `json:"reasoning"`    // 이유
}
```

---

## 실행 모드

### MCP Stdio 모드 (권장)

Claude Code에서 자동으로 실행되는 모드입니다.

**실행:**
```bash
./mcp-gorchestrator-server mcp
```

**특징:**
- JSON-RPC 2.0 프로토콜 사용
- stdin/stdout 통신
- Claude Code가 자동 관리
- 백그라운드 실행

### HTTP 서버 모드

개발 및 테스트용 모드입니다.

**실행:**
```bash
./mcp-gorchestrator-server
```

**엔드포인트:**
- POST `/orchestrate`
- 포트: 8080

**요청 예시:**
```bash
curl -X POST http://localhost:8080/orchestrate \
  -H "Content-Type: application/json" \
  -d '{
    "question": "이 코드를 리뷰해주세요",
    "claude_opinion": "이 코드는..."
  }'
```

---

## 워크플로우

### 전체 프로세스

```
1. Claude Code에서 ask_ai_consensus 호출
   ↓
2. MCP 서버 요청 수신
   ↓
3. RunConsensusWorkflow 실행
   ├─→ Gemini API 호출 (병렬)
   └─→ GPT-4 API 호출 (병렬)
   ↓
4. 의견 수집
   ├─→ 3개 모두 성공: 투표 진행
   ├─→ 2개만 성공: 간단한 비교
   └─→ 1개 이하: 에러 반환
   ↓
5. [투표 모드] ConductVoting
   ├─→ Gemini가 Claude vs GPT-4 평가
   └─→ GPT-4가 Claude vs Gemini 평가
   ↓
6. 투표 집계 및 우승자 결정
   ↓
7. FormatOpinions로 마크다운 생성
   ↓
8. Claude Code에 결과 반환
```

### 에러 처리 플로우

```
API 호출 실패
   ↓
opinions.{Model}Error에 저장
   ↓
FormatOpinions가 에러 섹션 표시
   ↓
사용자에게 명확한 에러 메시지 전달
```

---

## 환경 변수

필수 환경 변수:

- `GEMINI_API_KEY`: Google Gemini API 키
- `OPENAI_API_KEY`: OpenAI API 키

`.env` 파일 예시:
```env
GEMINI_API_KEY=your_gemini_api_key_here
OPENAI_API_KEY=your_openai_api_key_here
```

---

## 디버그 로깅

현재 구현된 디버그 로그:

1. **Gemini API 호출:**
   - `[DEBUG] Calling Gemini API...`
   - `[DEBUG] Gemini API success, response length: {length}`
   - `[DEBUG] Gemini API failed: {error}`

2. **Gemini 평가:**
   - `[DEBUG] Gemini evaluation success, response length: {length}`

3. **HTTP 핸들러:**
   - `Received question: {question}`
   - `Claude opinion length: {length} chars`

---

## 버전 정보

- **서버 이름:** gorchestrator
- **버전:** 1.0.0
- **MCP 프로토콜 버전:** 2024-11-05
- **지원 모델:**
  - Claude (Claude Code 내장)
  - Gemini 2.0 Flash Exp
  - GPT-4o-mini

---

## 개선 예정 사항

현재 코드 리뷰에서 제안된 개선사항:

1. ✅ 빈 응답 체크 (완료)
2. ✅ 디버그 로깅 (완료)
3. ⏳ 모델명 상수화
4. ⏳ 입력 검증 강화
5. ⏳ 로그 패키지 사용 (fmt.Printf → log)
6. ⏳ Context 타임아웃 추가
7. ⏳ Whitespace 전용 응답 체크

---

## 참고 링크

- [MCP Protocol Documentation](https://modelcontextprotocol.io/)
- [Gemini API Docs](https://ai.google.dev/gemini-api/docs)
- [OpenAI API Docs](https://platform.openai.com/docs/api-reference)

---

**마지막 업데이트:** 2026-01-06
