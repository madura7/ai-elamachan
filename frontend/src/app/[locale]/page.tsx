import { useTranslations } from "next-intl";
import { Link } from "@/i18n/navigation";

export default function HomePage() {
  const t = useTranslations("home");
  return (
    <main className="container">
      <h1>{t("title")}</h1>
      <p className="muted">{t("tagline")}</p>
      <Link className="button" href="/sell/ai-assist">
        {t("startSelling")}
      </Link>
    </main>
  );
}
