require "test_helper"

class AdditionalCostsControllerTest < ActionDispatch::IntegrationTest
  setup do
    @additional_cost = additional_costs(:one)
    sign_in_as @additional_cost.vehicle.user
  end

  test "should get new" do
    get new_additional_cost_url
    assert_response :success
  end

  test "should create additional cost" do
    assert_difference("AdditionalCost.count") do
      post additional_costs_url, params: { additional_cost: { vehicle_id: @additional_cost.vehicle_id, occurred_on: @additional_cost.occurred_on, kind: @additional_cost.kind, cost: @additional_cost.cost } }
    end

    assert_redirected_to root_url
  end

  test "should reject creating a cost for another user's vehicle" do
    assert_no_difference("AdditionalCost.count") do
      post additional_costs_url, params: { additional_cost: { vehicle_id: vehicles(:two).id, occurred_on: Date.current, kind: "repair", cost: 20 } }
    end

    assert_redirected_to root_url
  end

  test "should update additional cost" do
    patch additional_cost_url(@additional_cost), params: { additional_cost: { vehicle_id: @additional_cost.vehicle_id, occurred_on: @additional_cost.occurred_on, kind: "repair", cost: @additional_cost.cost } }
    assert_redirected_to root_url
    assert_equal "repair", @additional_cost.reload.kind
  end

  test "should reject moving a cost to another user's vehicle" do
    patch additional_cost_url(@additional_cost), params: { additional_cost: { vehicle_id: vehicles(:two).id, occurred_on: @additional_cost.occurred_on, kind: @additional_cost.kind, cost: @additional_cost.cost } }

    assert_redirected_to root_url
    assert_equal vehicles(:one).id, @additional_cost.reload.vehicle_id
  end

  test "should destroy additional cost" do
    assert_difference("AdditionalCost.count", -1) do
      delete additional_cost_url(@additional_cost)
    end

    assert_redirected_to root_url
  end
end
