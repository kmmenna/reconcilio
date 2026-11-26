import { createServer } from "node:http";

let submissions = 0;

createServer(async (request, response) => {
  if (request.method === "GET" && request.url === "/requests") {
    response.writeHead(200, { "Content-Type": "application/json" }).end(JSON.stringify({ submissions }));
    return;
  }
  if (request.method !== "POST" || request.url !== "/v1/responses") {
    response.writeHead(404).end();
    return;
  }
  submissions++;
  try {
    const chunks = await Array.fromAsync(request);
    const { input, model } = JSON.parse(Buffer.concat(chunks).toString("utf8"));
    const { candidates } = JSON.parse(input);
    const ranking = candidates.map(({ candidate_id, description }) => {
      const isCanva = description.includes("CANVA");
      return {
        candidate_id,
        score: isCanva ? 93 : 24,
        explanations: isCanva ? {
          "pt-BR": "As descrições mencionam a mesma assinatura do Canva.",
          en: "Both descriptions point to the same Canva subscription.",
        } : {
          "pt-BR": "O lançamento menciona um fornecedor diferente.",
          en: "The statement names a different merchant.",
        },
      };
    }).sort((left, right) => right.score - left.score);
    response.writeHead(200, { "Content-Type": "application/json" }).end(JSON.stringify({
      model,
      status: "completed",
      usage: { input_tokens: 50, output_tokens: 82 },
      output: [{ type: "message", role: "assistant", status: "completed", content: [
        { type: "output_text", text: JSON.stringify({ ranking }) },
      ] }],
    }));
  } catch {
    response.writeHead(400).end();
  }
}).listen(4010, "0.0.0.0");
