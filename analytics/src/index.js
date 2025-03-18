const uap = require("ua-parser-js");

export default {
  async fetch(request, env, ctx) {
    const corsHeaders = {
      "Access-Control-Allow-Origin": "*",
      "Access-Control-Allow-Methods": "GET,HEAD,POST,OPTIONS",
      "Access-Control-Allow-Headers": "*",
    };

    try {
      if (request.method === "GET") {
        const visitors = await env.DB.prepare(
          "SELECT date(created_at) as day, COUNT(1) AS count FROM visitors GROUP BY date(created_at) ORDER BY day DESC LIMIT 7",
        ).run();
        const url = await env.DB.prepare(
          "SELECT url, COUNT(*) AS count FROM visitors GROUP BY url ORDER BY count DESC LIMIT 10",
        ).run();
        const userAgentBrowser = await env.DB.prepare(
          "SELECT user_agent_browser, COUNT(*) AS count FROM visitors GROUP BY user_agent_browser ORDER BY count DESC LIMIT 10",
        ).run();
        const userAgentOS = await env.DB.prepare(
          "SELECT user_agent_os, COUNT(*) AS count FROM visitors GROUP BY user_agent_os ORDER BY count DESC LIMIT 10",
        ).run();
        const country = await env.DB.prepare(
          "SELECT country, COUNT(*) AS count FROM visitors GROUP BY country ORDER BY count DESC LIMIT 10",
        ).run();

        return Response.json(
          {
            visitors: visitors.results,
            url: url.results,
            userAgentBrowser: userAgentBrowser.results,
            userAgentOS: userAgentOS.results,
            country: country.results,
          },
          {
            status: 200,
            headers: {
              ...corsHeaders,
            },
          },
        );
      }

      if (request.method === "POST") {
        const userAgent = request.headers.get("user-agent");
        const referer = request.headers.get("referer");
        const city = request.cf.city;
        const continent = request.cf.continent;
        const country = request.cf.country;
        const latitude = request.cf.latitude;
        const longitude = request.cf.longitude;
        const postalCode = request.cf.postalCode;
        const region = request.cf.region;
        const regionCode = request.cf.regionCode;
        const timezone = request.cf.timezone;

        const body = await request.json();
        const ua = uap(userAgent);
        const userAgentBrowser = ua.browser.name;
        const userAgentOS = ua.os.name;

        if (referer != "https://ricoberger.de/") {
          return Response.json(
            { status: "error", error: "invalid referer" },
            {
              status: 500,
              headers: {
                ...corsHeaders,
              },
            },
          );
        }

        const { results } = await env.DB.prepare(
          "INSERT INTO visitors (url, referer, user_agent, user_agent_browser, user_agent_os, city, continent, country, latitude, longitude, postal_code, region, region_code, timezone) VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, ?12, ?13, ?14)",
        )
          .bind(
            body.url,
            referer,
            userAgent,
            userAgentBrowser,
            userAgentOS,
            city,
            continent,
            country,
            latitude,
            longitude,
            postalCode,
            region,
            regionCode,
            timezone,
          )
          .all();

        return Response.json(results, {
          headers: {
            status: 200,
            ...corsHeaders,
          },
        });
      }

      return Response.json(
        { status: "success" },
        {
          headers: {
            status: 200,
            ...corsHeaders,
          },
        },
      );
    } catch (err) {
      return Response.json(
        { status: "error", error: err.toString() },
        {
          status: 500,
          headers: {
            ...corsHeaders,
          },
        },
      );
    }
  },
};
