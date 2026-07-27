class CreateVehicles < ActiveRecord::Migration[8.1]
  def change
    create_table :vehicles do |t|
      t.references :user, null: false, foreign_key: true
      t.string :name, null: false
      t.integer :fuel_type, null: false

      t.timestamps
    end

    add_index :vehicles, [ :user_id, :name ]
  end
end
