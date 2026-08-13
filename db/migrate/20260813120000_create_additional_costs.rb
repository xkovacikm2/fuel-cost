class CreateAdditionalCosts < ActiveRecord::Migration[8.1]
  def change
    create_table :additional_costs do |t|
      t.references :vehicle, null: false, foreign_key: true
      t.date :occurred_on, null: false
      t.integer :kind, null: false
      t.decimal :cost, precision: 10, scale: 2, null: false

      t.timestamps
    end

    add_index :additional_costs, [ :vehicle_id, :occurred_on ]
  end
end
