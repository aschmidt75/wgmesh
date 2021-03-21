<template>
    <b-container id="main-container">
      <b-row>
        <b-col>
          <h3>Active mesh nodes</h3>
        </b-col>
        <b-col>
          <div class="float-left">
              <b-form>
                <b-form-input 
                  size="sm" 
                  class="mr" 
                  placeholder="Search in node data"
                  v-model="filter"
                  ></b-form-input>
              </b-form>
          </div>
          <div>
            <b-button class="float-right" v-if="withSystemTags" v-on:click="hideSystemTags" size="sm" variant="dark">hide system tags</b-button>
            <b-button class="float-right" v-else size="sm" v-on:click="showSystemTags" variant="outline-dark">show system tags</b-button>
          </div>
        </b-col>
      </b-row>
        <div v-if="items !== undefined && items.length">
          <b-table striped 
            :fields="fields" 
            :items="items"
            :sort-by.sync="sortBy"
            :sort-desc.sync="sortDesc" 
            :filter="filter"
            :filter-included-fields="__BVID__12"
            >
              <template #cell(tags)="data">
                <span v-html="data.value"></span>
              </template>
          </b-table>
        </div>
        <div v-else>
          <h5>No active nodes in mesh</h5>
        </div>
    </b-container>
</template>

<script>
import axios from "axios";
import { from } from 'rxjs';
import { map, toArray } from 'rxjs/operators';
const util = require ('util')

export default {
  name: 'NodesTable',
  data() {
    return {
          withSystemTags: false,
          sortBy: 'name',
          sortDesc: false,
          filter: null,
          filterOn: [],
          fields: [
            { key: "name", label: 'Node Name', sortable: true },
            { key: "meshIP", label: 'IP Address', sortable: true },
            { 
              key: "tags", 
              label: 'Tags', 
              sortable: false,
              filterByFormatted: true,
              formatter: (tag) => {
                let str = "<table>"
                for (let k in tag) {
                  if (this.withSystemTags == false && k.startsWith("_")) {
                    continue;
                  }
                  str += "<tr>"
                  const v = tag[k]
                  str +=  util.format("<td>%s</td><td>%s</td>", k, v);
                  str += "</tr>"
                }
                return str
              }
            },
          ], 
          items: []
    };
  },
  mounted() {
    this.items = this.getFromAPI()
  },
  methods: {
    hideSystemTags() {
      this.withSystemTags = false;
    },
    showSystemTags() {
      this.withSystemTags = true;
    },
    getMock() {
      this.items = [
            { "name": "m1", "meshIP": "10.0.0.1", "tags": { "_srv": "none", "_ip": "1.2.3.4" } },
            { "name": "m2", "meshIP": "10.0.12.34", "tags": { "_srv": "none" } }
      ]
    },
    getFromAPI() {
      axios
        .get("/api/nodes")
        .then(response => {
          if (response !== undefined && response.data !== undefined && response.data.nodes !== undefined) {
            const mappedData = from(response.data.nodes).pipe(
              map(elem => {
                return {
                  "name": elem.name,
                  "meshIP": elem.meshIP,
                  "tags": elem.tags
                }
              }),
              toArray()
            );
            mappedData.subscribe(res => {
              this.items = res
            })

          }
        })        
        .catch(err => {
          console.log(err);
        });
    }
  }
};
</script>


<style scoped>
#main-container > div {
  padding-top: 1rem;
}
</style>
