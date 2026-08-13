require "test_helper"

class DashboardControllerTest < ActionDispatch::IntegrationTest
  setup do
    sign_in_as users(:one)
  end

  test "should get show" do
    get root_url
    assert_response :success
  end

  test "shows additional costs in the dashboard chart payload and totals" do
    get root_url

    chart = css_select("div[data-controller='chart'][data-chart-payload-value]").first
    assert chart

    payload = JSON.parse(chart["data-chart-payload-value"])
    assert_equal [ "2026-07-27" ], payload.fetch("labels")
    assert_equal "2026-07-20", payload.fetch("additional_costs").first.fetch("date")
    assert_select "h3", text: /Záznamy tankovaní:/
    assert_select "h3", text: /Dodatočné náklady:/
    assert_includes response.body, "Výmena oleja"
  end

  test "adds mobile labels to refueling table cells" do
    get root_url

    assert_select "td[data-label='Dátum']", minimum: 1
    assert_select "td[data-label='Vozidlo']", minimum: 1
    assert_select "td[data-label='Akcie']", minimum: 1
  end

  test "combined dashboard includes costs from all user vehicles" do
    get root_url, params: { scope: "combined" }

    assert_response :success
    assert_includes response.body, "Výmena oleja"
  end

  test "vehicle dashboard excludes another vehicle's additional costs" do
    get root_url, params: { vehicle_id: vehicles(:one).id }

    assert_response :success
    assert_includes response.body, "Výmena oleja"
    assert_not_includes response.body, "Technická kontrola"
  end
end
