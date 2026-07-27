require "test_helper"

class RefuelingsControllerTest < ActionDispatch::IntegrationTest
  setup do
    @refueling = refuelings(:one)
    sign_in_as @refueling.user
  end

  test "should get new" do
    get new_refueling_url
    assert_response :success
  end

  test "should create refueling" do
    assert_difference("Refueling.count") do
      post refuelings_url, params: { refueling: { amount: @refueling.amount, cost: @refueling.cost, distance_km: @refueling.distance_km, refueled_on: @refueling.refueled_on, vehicle_id: @refueling.vehicle_id } }
    end

    assert_redirected_to root_url
  end

  test "should get edit" do
    get edit_refueling_url(@refueling)
    assert_response :success
  end

  test "should update refueling" do
    patch refueling_url(@refueling), params: { refueling: { amount: @refueling.amount, cost: @refueling.cost, distance_km: @refueling.distance_km, refueled_on: @refueling.refueled_on, vehicle_id: @refueling.vehicle_id } }
    assert_redirected_to root_url
  end

  test "should destroy refueling" do
    assert_difference("Refueling.count", -1) do
      delete refueling_url(@refueling)
    end

    assert_redirected_to root_url
  end
end
