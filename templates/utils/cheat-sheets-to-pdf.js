const puppeteer = require("puppeteer");
const fs = require("fs");

// Get all directories in the given source directory
const getDirectories = (source) => {
  return fs
    .readdirSync(source, { withFileTypes: true })
    .filter((dirent) => dirent.isDirectory())
    .map((dirent) => dirent.name);
};

// Create a PDF for the given cheat sheet
const createCheatSheet = async (cheatSheet) => {
  // Create a browser instance
  const browser = await puppeteer.launch({
    headless: true,
    args: ["--no-sandbox", "--disable-setuid-sandbox"],
  });

  // Create a new page
  const page = await browser.newPage();

  // Set the size of the new page
  await page.setViewport({ width: 1920, height: 1080, deviceScaleFactor: 1 });

  // Website URL to export as pdf
  const website_url = "http://localhost:9999/cheat-sheets/" + cheatSheet + "/";

  // Open URL in current page
  await page.goto(website_url, { waitUntil: "networkidle0" });

  // Remove header before pdf is generated
  await page.evaluate((sel) => {
    var elements = document.querySelectorAll(sel);
    for (var i = 0; i < elements.length; i++) {
      elements[i].parentNode.removeChild(elements[i]);
    }
  }, "#header");

  // To reflect CSS used for screens instead of print
  await page.emulateMediaType("screen");

  // Get the height of the page
  const pageHeight = await page.evaluate(
    () => document.documentElement.offsetHeight,
  );

  // Downlaod the PDF
  await page.pdf({
    path:
      "dist/cheat-sheets/" +
      cheatSheet +
      "/assets/" +
      cheatSheet +
      "-cheat-sheet.pdf",
    printBackground: true,
    width: "1920px",
    height: pageHeight + "px",
  });

  // Close the browser instance
  await browser.close();
};

// Get all cheat sheets from the dist directory and ensure that there is an
// assets directory. Afterwards create the pdf version of the cheat sheet and
// save it in the existing / created assets directory.
(async () => {
  const cheatSheets = getDirectories("dist/cheat-sheets");

  for (const cheatSheet of cheatSheets) {
    console.log("Process " + cheatSheet + "...");

    if (!fs.existsSync("dist/cheat-sheets/" + cheatSheet + "/assets/")) {
      fs.mkdirSync("dist/cheat-sheets/" + cheatSheet + "/assets/", {
        recursive: true,
      });
    }

    await createCheatSheet(cheatSheet);
  }
})();
