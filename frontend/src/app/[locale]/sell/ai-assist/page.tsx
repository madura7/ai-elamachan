import { useTranslations } from "next-intl";
import { AiAssistEditor } from "./AiAssistEditor";

export default function AiAssistPage() {
  const t = useTranslations("aiAssist");
  return (
    <main className="container">
      <h1>{t("heading")}</h1>
      <p className="muted">{t("intro")}</p>
      <AiAssistEditor />
    </main>
  );
}
