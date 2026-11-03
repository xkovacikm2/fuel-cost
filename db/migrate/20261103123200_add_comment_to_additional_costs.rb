class AddCommentToAdditionalCosts < ActiveRecord::Migration[8.1]
  def change
    add_column :additional_costs, :comment, :string, limit: 255
  end
end
