require "test_helper"

class AdditionalCostTest < ActiveSupport::TestCase
  test "exposes Slovak labels for cost kinds" do
    assert_equal "Výmena oleja", additional_costs(:one).kind_label
    assert_includes AdditionalCost.kind_options, [ "Technická kontrola", "technical_inspection" ]
  end

  test "for_user only returns costs belonging to that user" do
    assert_equal [ additional_costs(:one) ], AdditionalCost.for_user(users(:one)).to_a
  end

  test "requires a positive cost and a date" do
    additional_cost = AdditionalCost.new(vehicle: vehicles(:one), kind: :repair, cost: 0)

    assert_not additional_cost.valid?
    assert_includes additional_cost.errors[:occurred_on], "can't be blank"
    assert_includes additional_cost.errors[:cost], "must be greater than 0"
  end

  test "within_range scopes by occurrence date and ordered sorts newest first" do
    assert_equal [ additional_costs(:one) ], AdditionalCost.within_range(Date.new(2026, 7, 20), Date.new(2026, 7, 20)).ordered.to_a
  end
end
