const messagesEl = document.getElementById("messages");
const toolCallsEl = document.getElementById("toolCalls");
const providerInfoEl = document.getElementById("providerInfo");
const transcriptionInfoEl = document.getElementById("transcriptionInfo");
const form = document.getElementById("chatForm");
const userInput = document.getElementById("userInput");
const agentSelect = document.getElementById("agentSelect");
const audioInput = document.getElementById("audioInput");
const imageInput = document.getElementById("imageInput");
const reloadAgentsBtn = document.getElementById("reloadAgents");

const history = [];

function authHeaders(extra = {}) {
  const headers = { ...extra };
  const apiKey = document.getElementById("apiKey").value.trim();
  if (apiKey) headers["X-API-Key"] = apiKey;
  return headers;
}

function appendMessage(role, content) {
  const div = document.createElement("div");
  div.className = `msg msg-${role}`;
  div.textContent = typeof content === "string" ? content : JSON.stringify(content);
  messagesEl.appendChild(div);
  messagesEl.scrollTop = messagesEl.scrollHeight;
}

function agentLabel(ag) {
  const models = [ag.llmModel, ag.visionModel].filter(Boolean).join(" / ");
  return `${ag.name}${models ? ` (${models})` : ""}${ag.enabled ? "" : " (disabled)"}`;
}

async function loadAgents() {
  agentSelect.innerHTML = '<option value="">Loading…</option>';
  try {
    const res = await fetch("/api/agents", { headers: authHeaders() });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || res.statusText);
    agentSelect.innerHTML = "";
    if (!data.length) {
      agentSelect.innerHTML = '<option value="">No agents — run make seed</option>';
      return;
    }
    for (const ag of data) {
      const opt = document.createElement("option");
      opt.value = ag.id;
      opt.textContent = agentLabel(ag);
      agentSelect.appendChild(opt);
    }
  } catch (err) {
    agentSelect.innerHTML = `<option value="">Error: ${err.message}</option>`;
  }
}

reloadAgentsBtn.addEventListener("click", loadAgents);
loadAgents();

form.addEventListener("submit", async (e) => {
  e.preventDefault();
  const text = userInput.value.trim();
  const audioFile = audioInput.files[0];
  const imageFile = imageInput.files[0];
  const agentId = agentSelect.value;

  if (!agentId) {
    alert("Select an agent first");
    return;
  }
  if (!text && !audioFile && !imageFile) {
    alert("Enter text, voice, or image");
    return;
  }

  const useMultipart = !!(audioFile || imageFile);
  if (text && !useMultipart) {
    history.push({ role: "user", content: text });
    appendMessage("user", text);
  } else {
    const parts = [];
    if (text) parts.push(text);
    if (audioFile) parts.push(`[audio: ${audioFile.name}]`);
    if (imageFile) parts.push(`[image: ${imageFile.name}]`);
    appendMessage("user", parts.join(" "));
  }

  userInput.value = "";
  audioInput.value = "";
  imageInput.value = "";

  const btn = form.querySelector('button[type="submit"]');
  btn.disabled = true;
  transcriptionInfoEl.textContent = "";

  try {
    let res;
    if (useMultipart) {
      const body = new FormData();
      if (audioFile) body.append("audio", audioFile);
      if (imageFile) body.append("image", imageFile);
      if (text) body.append("text", text);
      body.append("messages", JSON.stringify(history));
      res = await fetch(`/api/agents/${agentId}/chat`, {
        method: "POST",
        headers: authHeaders(),
        body,
      });
    } else {
      res = await fetch(`/api/agents/${agentId}/chat`, {
        method: "POST",
        headers: authHeaders({ "Content-Type": "application/json" }),
        body: JSON.stringify({ messages: history }),
      });
    }

    const data = await res.json();
    if (!res.ok) {
      throw new Error(data.error || res.statusText);
    }

    if (data.transcription) {
      history.push({ role: "user", content: data.transcription });
      transcriptionInfoEl.textContent = `Transcription: ${data.transcription}`;
    } else if (useMultipart && (text || imageFile)) {
      if (imageFile && text) {
        history.push({
          role: "user",
          content: [{ type: "text", text }, { type: "image_url", image_url: { url: "(uploaded)" } }],
        });
      } else if (text) {
        history.push({ role: "user", content: text });
      }
    }
    history.push(data.message);
    appendMessage("assistant", data.message.content || "(empty)");
    providerInfoEl.textContent = `Endpoint: ${data.endpoint} · Model: ${data.model}`;
    toolCallsEl.textContent = JSON.stringify(data.toolCalls || [], null, 2);
  } catch (err) {
    appendMessage("assistant", `Error: ${err.message}`);
  } finally {
    btn.disabled = false;
    userInput.focus();
  }
});
