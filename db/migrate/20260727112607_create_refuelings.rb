class CreateRefuelings < ActiveRecord::Migration[8.1]
  def change
    create_table :refuelings do |t|
      t.references :vehicle, null: false, foreign_key: true
      t.date :refueled_on, null: false
      t.decimal :distance_km, precision: 10, scale: 2, null: false
      t.decimal :amount, precision: 10, scale: 2, null: false
      t.decimal :cost, precision: 10, scale: 2, null: false

      t.timestamps
    end

    add_index :refuelings, [ :vehicle_id, :refueled_on ]
  end
end
