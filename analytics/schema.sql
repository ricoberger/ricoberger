CREATE TABLE IF NOT EXISTS "visitors" (
  "id" INTEGER PRIMARY KEY AUTOINCREMENT,
  "url" TEXT DEFAULT NULL,
  "referer" TEXT DEFAULT NULL,
  "user_agent" TEXT DEFAULT NULL,
  "user_agent_browser" TEXT DEFAULT NULL,
  "user_agent_os" TEXT DEFAULT NULL,
  "city" TEXT DEFAULT NULL,
  "continent" TEXT DEFAULT NULL,
  "country" TEXT DEFAULT NULL,
  "latitude" DECIMAL DEFAULT NULL,
  "longitude" DECIMAL DEFAULT NULL,
  "postal_code" TEXT DEFAULT NULL,
  "region" TEXT DEFAULT NULL,
  "region_code" TEXT DEFAULT NULL,
  "timezone" TEXT DEFAULT NULL,
  "created_at" DATETIME DEFAULT CURRENT_TIMESTAMP
);
